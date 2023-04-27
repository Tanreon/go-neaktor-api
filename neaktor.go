package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/ratelimit"

	HttpRunner "github.com/Tanreon/go-http-runner"
	log "github.com/sirupsen/logrus"
)

const API_SERVER = "https://api.neaktor.com"
const MODEL_CACHE_TIME = time.Minute * 30

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
	runner       HttpRunner.IHttpRunner
	refreshToken string
	token        string

	modelCacheLock sync.Mutex
	modelCacheMap  map[string]ModelCache
}

type INeaktor interface {
	RefreshToken(clientId, clientSecret, refreshToken string) (err error)
	GetModelByTitle(title string) (model IModel, err error)
	MustGetModelByTitle(title string) (model IModel)
}

func NewNeaktor(runner *HttpRunner.IHttpRunner, apiToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter:     ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		runner:         *runner,
		token:          apiToken,
		modelCacheLock: sync.Mutex{},
		modelCacheMap:  make(map[string]ModelCache, 0),
	}
}

func NewNeaktorByRefreshToken(runner *HttpRunner.IHttpRunner, refreshToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter:     ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		runner:         *runner,
		refreshToken:   refreshToken,
		modelCacheLock: sync.Mutex{},
		modelCacheMap:  make(map[string]ModelCache, 0),
	}
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

	formRequestOptions := HttpRunner.NewFormRequestOptions("https://api.neaktor.com/oauth/token")
	formRequestOptions.SetValues(map[string]string{
		"grant_type":    "refresh_token",
		"redirect_uri":  "https://redirectUri.com",
		"client_id":     clientId,
		"client_secret": clientSecret,
		"refresh_token": refreshToken,
	})

	response, err := n.runner.PostForm(formRequestOptions)
	if err != nil {
		return fmt.Errorf("/oauth/token response error: %w", err)
	}
	if response.StatusCode() >= 500 {
		log.Debugf("response status code: %d", response.StatusCode())
		return fmt.Errorf("service unavailable, code: %d", response.StatusCode())
	}

	var oauthTokenResponse OauthTokenResponse
	if err := json.Unmarshal(response.Body(), &oauthTokenResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(oauthTokenResponse.Code) > 0 {
		parseErrorCode(oauthTokenResponse.Code, oauthTokenResponse.Message)
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
		if time.Now().Before(cachedModel.lastUpdatedAt.Add(MODEL_CACHE_TIME)) {
			return cachedModel.model, err
		}

		delete(n.modelCacheMap, title)
	}

	// request second

	n.apiLimiter.Take()

	jsonRequestOptions := HttpRunner.NewJsonRequestOptions(API_SERVER + "/v1/taskmodels?size=100")
	jsonRequestOptions.SetHeaders(map[string]string{
		"Authorization": n.token,
	})

	response, err := n.runner.GetJson(jsonRequestOptions)
	if err != nil {
		return model, fmt.Errorf("/v1/taskmodels?size=100 response error: %w", err)
	}
	if response.StatusCode() >= 500 {
		log.Debugf("response status code: %d", response.StatusCode())
		return model, fmt.Errorf("service unavailable, code: %d", response.StatusCode())
	}

	var taskModelResponse TaskModelResponse
	if err := json.Unmarshal(response.Body(), &taskModelResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
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
