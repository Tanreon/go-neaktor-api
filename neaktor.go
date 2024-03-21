package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/wangluozhe/requests"
	"go.uber.org/ratelimit"

	requrl "github.com/wangluozhe/requests/url"
)

const ApiServer = "https://api.neaktor.com"
const ApiGateway = ApiServer + "/v1"
const ModelCacheTime = time.Minute * 30

var ErrCodeUnknown = errors.New("UNKNOWN_ERROR")
var ErrCode403 = errors.New("403")
var ErrCode404 = errors.New("404")
var ErrCode422 = errors.New("422")
var ErrCode429 = errors.New("429 TOO_MANY_REQUESTS") // This and below not typo error
var ErrCode500 = errors.New("500 INTERNAL_SERVER_ERROR")
var ErrApiTokenIncorrect = errors.New("API_TOKEN_INCORRECT")
var ErrModelNotFound = errors.New("MODEL_NOT_FOUND")

type ModelCache struct {
	lastUpdatedAt time.Time
	model         IModel
}

type NeaktorErrorResponse struct {
	Type             string `json:"type"` // error type
	Message          string `json:"message"`
	Code             string `json:"code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type Neaktor struct {
	apiLimiter   ratelimit.Limiter
	httpClient   requrl.Request
	refreshToken string
	token        string

	log *log.Logger

	modelCacheLock sync.Mutex
	modelCacheMap  map[string]ModelCache
}

type INeaktor interface {
	RefreshToken(clientId, clientSecret, refreshToken string) (err error)
	GetModelByTitle(title string) (model IModel, err error)
	MustGetModelByTitle(title string) (model IModel)
	SetLogger(log *log.Logger)
}

func NewNeaktor(httpClient requrl.Request, apiToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter: ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		httpClient: httpClient,
		token:      apiToken,
		log:        log.WithPrefix("neaktor"),

		modelCacheLock: sync.Mutex{},
		modelCacheMap:  make(map[string]ModelCache, 0),
	}
}

func NewNeaktorByRefreshToken(httpClient requrl.Request, refreshToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter:   ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		httpClient:   httpClient,
		refreshToken: refreshToken,
		log:          log.WithPrefix("neaktor"),

		modelCacheLock: sync.Mutex{},
		modelCacheMap:  make(map[string]ModelCache, 0),
	}
}

func (n *Neaktor) SetLogger(logger *log.Logger) {
	n.log = logger
}

func (n *Neaktor) RefreshToken(clientId, clientSecret, refreshToken string) (err error) { // FIXME временная мера из-за бага в самом неакторе, приходится таким образом доставать ключ
	type OauthTokenResponse struct {
		NeaktorErrorResponse
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	httpClient := n.httpClient

	httpClient.Data = requrl.NewData()
	httpClient.Data.Add("grant_type", "refresh_token")
	httpClient.Data.Add("redirect_uri", "https://redirectUri.com")
	httpClient.Data.Add("client_id", clientId)
	httpClient.Data.Add("client_secret", clientSecret)
	httpClient.Data.Add("refresh_token", refreshToken)

	response, err := requests.Post(mustUrlJoinPath(ApiServer, "oauth", "token"), &httpClient)
	if err != nil {
		return fmt.Errorf("/oauth/token request error: %w", err)
	}

	if response.StatusCode >= 500 {
		n.log.Debugf("response status code: %d", response.StatusCode)
		return fmt.Errorf("service unavailable, code: %d", response.StatusCode)
	}

	var oauthTokenResponse OauthTokenResponse
	if err := json.Unmarshal(response.Content, &oauthTokenResponse); err != nil {
		n.log.Debugf("response code: %d, response body: %v", response.StatusCode, response.Text)
		return fmt.Errorf("unmarshaling error: %w", err)
	}

	if len(oauthTokenResponse.Code) > 0 {
		return parseErrorCode(oauthTokenResponse.Code, oauthTokenResponse.Message)
	}

	if len(oauthTokenResponse.AccessToken) <= 0 {
		return ErrApiTokenIncorrect
	}

	n.token = "Bearer " + oauthTokenResponse.AccessToken

	return err
}

func (n *Neaktor) GetModelByTitle(title string) (model IModel, err error) {
	type TaskModelResponseDataFields struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	}
	type TaskModelResponseDataStatuses struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Closed bool   `json:"closed"`
		Type   string `json:"type"`
	}
	type TaskModelResponseDataRoles struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	type TaskModelResponseData struct {
		Id               string                          `json:"id"`
		Name             string                          `json:"name"`
		CreatedBy        int                             `json:"createdBy"`
		LastModifiedBy   *int                            `json:"lastModifiedBy"`
		CreatedDate      string                          `json:"createdDate"`
		LastModifiedDate *string                         `json:"lastModifiedDate"`
		Fields           []TaskModelResponseDataFields   `json:"fields"`
		Statuses         []TaskModelResponseDataStatuses `json:"statuses"`
		StartStatus      string                          `json:"startStatus"`
		CanCreateTask    bool                            `json:"canCreateTask"`
		ModuleId         string                          `json:"moduleId"`
		Roles            []TaskModelResponseDataRoles    `json:"roles"`
		DeadlineStatus   string                          `json:"deadlineStatus"`
	}
	type TaskModelResponse struct {
		Data  []TaskModelResponseData `json:"data"`
		Page  int                     `json:"page"`
		Size  int                     `json:"size"`
		Total int                     `json:"total"`
		NeaktorErrorResponse
	}

	n.modelCacheLock.Lock()
	defer n.modelCacheLock.Unlock()

	// cache first

	if cachedModel, present := n.modelCacheMap[title]; present {
		if time.Now().Before(cachedModel.lastUpdatedAt.Add(ModelCacheTime)) {
			return cachedModel.model, err
		}

		delete(n.modelCacheMap, title)
	}

	// request second

	n.apiLimiter.Take()

	httpClient := n.httpClient

	httpClient.Headers = requrl.NewHeaders()
	httpClient.Headers.Add("Authorization", n.token)

	httpClient.Params = requrl.NewParams()
	httpClient.Params.Add("size", "100")

	response, err := requests.Get(mustUrlJoinPath(ApiGateway, "taskmodels"), &httpClient)
	if err != nil {
		return model, fmt.Errorf("/v1/taskmodels request error: %w", err)
	}

	if response.StatusCode >= 500 {
		n.log.Debugf("response status code: %d", response.StatusCode)
		return model, fmt.Errorf("service unavailable, code: %d", response.StatusCode)
	}

	var taskModelResponse TaskModelResponse
	if err := json.Unmarshal(response.Content, &taskModelResponse); err != nil {
		n.log.Debugf("response code: %d, response body: %v", response.StatusCode, response.Text)
		return model, fmt.Errorf("unmarshaling error: %w", err)
	}

	if len(taskModelResponse.Code) > 0 {
		return model, parseErrorCode(taskModelResponse.Code, taskModelResponse.Message)
	}

	for _, item := range taskModelResponse.Data {
		modelStatuses := make(map[string]ModelStatus, 0)
		for _, status := range item.Statuses {
			modelStatuses[status.Id] = ModelStatus{
				Id:     status.Id,
				Name:   status.Name,
				Closed: status.Closed,
				Type:   status.Type,
			}
		}
		modelFields := make(map[string]ModelField, 0)
		for _, field := range item.Fields {
			modelFields[field.Id] = ModelField{
				Id:    field.Id,
				Name:  field.Name,
				State: field.State,
			}
		}

		model = NewModel(n, item.Id, modelStatuses, modelFields)

		n.modelCacheMap[item.Name] = ModelCache{
			lastUpdatedAt: time.Now(),
			model:         model,
		}
	}

	//

	if cachedModel, present := n.modelCacheMap[title]; present {
		return cachedModel.model, err
	}

	return model, ErrModelNotFound
}

func (n *Neaktor) MustGetModelByTitle(title string) (model IModel) {
	var err error
	model, err = n.GetModelByTitle(title)
	if err != nil {
		panic(err)
	}

	return model
}
