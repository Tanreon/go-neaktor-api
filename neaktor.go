package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
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

	modelCacheMap map[string]ModelCache
}

type INeaktor interface {
	GetModelByTitle(title string) (model IModel, err error)
}

func NewNeaktor(runner *HttpRunner.IHttpRunner, apiToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter:    ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		runner:        *runner,
		token:         apiToken,
		modelCacheMap: make(map[string]ModelCache, 0),
	}
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

	// cache first

	if cachedModel, present := n.modelCacheMap[title]; present {
		if time.Now().Before(cachedModel.lastUpdatedAt.Add(MODEL_CACHE_TIME)) {
			return cachedModel.model, err
		}

		delete(n.modelCacheMap, title)
	}

	// request second

	n.apiLimiter.Take()

	jsonRequestData := HttpRunner.NewJsonRequestData(API_SERVER + "/v1/taskmodels?size=100")
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": n.token,
	})

	response, err := n.runner.GetJson(jsonRequestData)
	if err != nil {
		return model, fmt.Errorf("/v1/taskmodels?size=100 response error: %w", err)
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
		if item.Name == title {
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

			return model, err
		}
	}

	return model, ErrModelNotFound
}
