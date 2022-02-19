package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/ratelimit"

	HttpRunner "github.com/Tanreon/go-http-runner"
	log "github.com/sirupsen/logrus"
)

const API_SERVER = "https://api.neaktor.com"
const MODEL_CACHE_TIME = time.Minute * 30

var ErrCodeUnknown = errors.New("UNKNOWN_ERROR")
var ErrCode403 = errors.New("403 FORBIDDEN")
var ErrCode404 = errors.New("404 NOT_FOUND")
var ErrCode422 = errors.New("422 UNPROCESSABLE_ENTITY")
var ErrCode429 = errors.New("429 TOO_MANY_REQUESTS")
var ErrCode500 = errors.New("500 INTERNAL_SERVER_ERROR")

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
	//GetModelInfo(title string) (modelId string, statusId []ModelStatus, modelFields []ModelField, err error)
	//GetModelRoutingsByModel(modelId, statusId string) (userId int, err error)
	//CanUpdateModel() (canUpdate bool)

	//CreateCommentByTask(taskId int, comment string) (err error)
	//
	//GetModelTasksByStatus(modelId, statusId string) (tasks []TasksResponseData, err error)
	//GetModelTasksByStatuses(modelId string, statusId []string) (tasks []TasksResponseData, err error)
	//GetModelTasksByStatusAndFields(modelId, statusId string, dataFields []TaskDataField) (tasks []TasksResponseData, err error)
	//GetModelTasksByFields(modelId string, dataFields []TaskDataField) (tasks []TasksResponseData, err error)
	//UpdateTask(taskId int, fields []UpdateTaskField) (err error)
	//UpdateTaskStatus(taskId int, statusId string) (err error)
}

func NewNeaktor(runner *HttpRunner.IHttpRunner, apiToken string, apiLimit int) INeaktor {
	return &Neaktor{
		apiLimiter:    ratelimit.New(apiLimit, ratelimit.Per(time.Minute)),
		runner:        *runner,
		token:         apiToken,
		modelCacheMap: make(map[string]ModelCache, 0),
	}
}

func parseErrorCode(code string, message string) error {
	if strings.EqualFold(code, ErrCode403.Error()) {
		return fmt.Errorf("%w: %s", ErrCode403, message)
	}
	if strings.EqualFold(code, ErrCode404.Error()) {
		return fmt.Errorf("%w: %s", ErrCode404, message)
	}
	if strings.EqualFold(code, ErrCode429.Error()) {
		return fmt.Errorf("%w: %s", ErrCode429, message)
	}
	if strings.EqualFold(code, ErrCode422.Error()) {
		return fmt.Errorf("%w: %s", ErrCode422, message)
	}
	if strings.EqualFold(code, ErrCode500.Error()) {
		return fmt.Errorf("%w: %s", ErrCode500, message)
	}

	return ErrCodeUnknown
}

////////

var ErrModelNotFound = errors.New("MODEL_NOT_FOUND")

type ModelCache struct {
	lastUpdatedAt time.Time
	model         IModel
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

	response, err := n.runner.GetJson(HttpRunner.JsonRequestData{
		Url: API_SERVER + "/v1/taskmodels?size=100",
		Headers: map[string]string{
			"Authorization": n.token,
		},
	})
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
