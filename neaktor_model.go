package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"

	HttpRunner "github.com/Tanreon/go-http-runner"
	log "github.com/sirupsen/logrus"
)

type ModelField struct {
	Id    string
	Name  string
	State string
}

type ModelStatus struct {
	Id     string
	Name   string
	Closed bool
	Type   string
}

type ModelRoles struct {
	Id   string
	Name string
}

type ModelAssignee struct {
	Id   string
	Name string
	Type string
}

type Model struct {
	neaktor  *Neaktor
	modelId  string
	statuses map[string]ModelStatus
	fields   map[string]ModelField
}

var ErrModelStatusNotFound = errors.New("MODEL_STATUS_NOT_FOUND")
var ErrModelFieldNotFound = errors.New("MODEL_FIELD_NOT_FOUND")

type IModel interface {
	GetStatus(title string) (status ModelStatus, err error)
	GetField(title string) (field ModelField, err error)
	GetStatuses() (statuses map[string]ModelStatus)
	GetFields() (fields map[string]ModelField)
	GetTasksByStatus(status ModelStatus) (tasks []ITask, err error)
	GetTasksByStatuses(statuses []ModelStatus) (tasks []ITask, err error)
	GetTasksByStatusAndFields(status ModelStatus, dataFields []TaskField) (tasks []ITask, err error)
	GetTasksByFields(dataFields []TaskField) (tasks []ITask, err error)
}

func NewModel(neaktor *Neaktor, modelId string, statuses map[string]ModelStatus, fields map[string]ModelField) IModel {
	return &Model{
		neaktor:  neaktor,
		modelId:  modelId,
		statuses: statuses,
		fields:   fields,
	}
}

func (m *Model) GetStatus(title string) (status ModelStatus, err error) {
	for _, modelStatus := range m.statuses {
		if strings.EqualFold(modelStatus.Name, title) {
			return modelStatus, err
		}
	}

	return status, ErrModelStatusNotFound
}

func (m *Model) GetField(title string) (field ModelField, err error) {
	for _, modelField := range m.fields {
		if strings.EqualFold(modelField.Name, title) {
			return modelField, err
		}
	}

	return field, ErrModelFieldNotFound
}

func (m *Model) GetStatuses() (statuses map[string]ModelStatus) {
	return m.statuses
}

func (m *Model) GetFields() (fields map[string]ModelField) {
	return m.fields
}

//

func (m *Model) GetTasksByStatus(status ModelStatus) (tasks []ITask, err error) {
	type TaskDataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int             `json:"id"`
		ProjectId  string          `json:"projectId"`
		Fields     []TaskDataField `json:"fields"`
		Status     string          `json:"status"`
		ModelId    string          `json:"modelId"`
		CanDelete  bool            `json:"canDelete"`
		ModuleId   string          `json:"moduleId"`
		Idx        string          `json:"idx"`
		ParentId   interface{}     `json:"parentId"`
		SubtaskIds []interface{}   `json:"subtaskIds"`
	}

	type TasksResponseLinks struct {
		Next string `json:"next"`
	}

	type TasksResponse struct {
		NeaktorErrorResponse
		Data  []TasksResponseData `json:"data"`
		Links TasksResponseLinks  `json:"links"`
		Page  int                 `json:"page"`
		Size  int                 `json:"size"`
		Total int                 `json:"total"`
	}

	//

	limit := 50
	maxPages := 1

	for page := 0; page < maxPages; page++ {
		m.neaktor.apiLimiter.Take()

		response, err := m.neaktor.runner.GetJson(HttpRunner.JsonRequestData{
			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d", m.modelId, status.Id, limit, page),
			Value: nil,
			Headers: map[string]string{
				"Authorization": m.neaktor.token,
			},
		})
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d response error: %w", m.modelId, status.Id, limit, page, err)
		}

		var tasksResponse TasksResponse
		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
			return tasks, fmt.Errorf("unmarshaling error: %w", err)
		}
		if len(tasksResponse.Code) > 0 {
			return tasks, parseErrorCode(tasksResponse.Code, tasksResponse.Message)
		}

		for _, data := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			for _, field := range data.Fields {
				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			tasks = append(tasks, NewTask(m, data.Id, data.Idx, fields))
		}

		//

		maxPages = int(math.Ceil(float64(tasksResponse.Total) / float64(limit)))
	}

	return tasks, err
}

func (m *Model) GetTasksByStatuses(statuses []ModelStatus) (tasks []ITask, err error) {
	for _, status := range statuses {
		tasksByStatus, err := m.GetTasksByStatus(status)
		if err != nil {
			return tasks, err
		}

		tasks = append(tasks, tasksByStatus...)
	}

	return tasks, err
}

func (m *Model) GetTasksByStatusAndFields(status ModelStatus, dataFields []TaskField) (tasks []ITask, err error) {
	type TaskDataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int             `json:"id"`
		ProjectId  string          `json:"projectId"`
		Fields     []TaskDataField `json:"fields"`
		Status     string          `json:"status"`
		ModelId    string          `json:"modelId"`
		CanDelete  bool            `json:"canDelete"`
		ModuleId   string          `json:"moduleId"`
		Idx        string          `json:"idx"`
		ParentId   interface{}     `json:"parentId"`
		SubtaskIds []interface{}   `json:"subtaskIds"`
	}

	type TasksResponseLinks struct {
		Next string `json:"next"`
	}

	type TasksResponse struct {
		NeaktorErrorResponse
		Data  []TasksResponseData `json:"data"`
		Links TasksResponseLinks  `json:"links"`
		Page  int                 `json:"page"`
		Size  int                 `json:"size"`
		Total int                 `json:"total"`
	}

	//

	values := url.Values{}
	for _, field := range dataFields {
		var value string
		switch field.Value.(type) {
		case string:
			value = field.Value.(string)
		case float64:
			value = fmt.Sprintf("%f", field.Value.(float64))
		case float32:
			value = fmt.Sprintf("%f", field.Value.(float32))
		case int:
			value = fmt.Sprintf("%field", field.Value.(int))
		case int8:
			value = fmt.Sprintf("%field", field.Value.(int8))
		case int16:
			value = fmt.Sprintf("%field", field.Value.(int16))
		case int32:
			value = fmt.Sprintf("%field", field.Value.(int32))
		case int64:
			value = fmt.Sprintf("%field", field.Value.(int64))
		}
		values.Add(field.ModelField.Id, value)
	}

	page := 0

	for {
		m.neaktor.apiLimiter.Take()

		response, err := m.neaktor.runner.GetJson(HttpRunner.JsonRequestData{
			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&%s&size=50&page=%d", m.modelId, status.Id, values.Encode(), page),
			Value: nil,
			Headers: map[string]string{
				"Authorization": m.neaktor.token,
			},
		})
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&%s&size=%d response error: %w", m.modelId, status.Id, values.Encode(), page, err)
		}

		var tasksResponse TasksResponse
		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
			return tasks, fmt.Errorf("unmarshaling error: %w", err)
		}
		if len(tasksResponse.Code) > 0 {
			return tasks, parseErrorCode(tasksResponse.Code, tasksResponse.Message)
		}

		for _, data := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			for _, field := range data.Fields {
				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			tasks = append(tasks, NewTask(m, data.Id, data.Idx, fields))
		}

		if tasksResponse.Total < 50 {
			break
		}

		if float64(page) >= math.Ceil(float64(tasksResponse.Total/50)) {
			break
		}

		page++
	}

	return tasks, err
}

func (m *Model) GetTasksByFields(dataFields []TaskField) (tasks []ITask, err error) {
	type TaskDataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int             `json:"id"`
		ProjectId  string          `json:"projectId"`
		Fields     []TaskDataField `json:"fields"`
		Status     string          `json:"status"`
		ModelId    string          `json:"modelId"`
		CanDelete  bool            `json:"canDelete"`
		ModuleId   string          `json:"moduleId"`
		Idx        string          `json:"idx"`
		ParentId   interface{}     `json:"parentId"`
		SubtaskIds []interface{}   `json:"subtaskIds"`
	}

	type TasksResponseLinks struct {
		Next string `json:"next"`
	}

	type TasksResponse struct {
		NeaktorErrorResponse
		Data  []TasksResponseData `json:"data"`
		Links TasksResponseLinks  `json:"links"`
		Page  int                 `json:"page"`
		Size  int                 `json:"size"`
		Total int                 `json:"total"`
	}

	//

	values := url.Values{}
	for _, field := range dataFields {
		var value string
		switch field.Value.(type) {
		case string:
			value = field.Value.(string)
		case float64:
			value = fmt.Sprintf("%f", field.Value.(float64))
		case float32:
			value = fmt.Sprintf("%f", field.Value.(float32))
		case int:
			value = fmt.Sprintf("%field", field.Value.(int))
		case int8:
			value = fmt.Sprintf("%field", field.Value.(int8))
		case int16:
			value = fmt.Sprintf("%field", field.Value.(int16))
		case int32:
			value = fmt.Sprintf("%field", field.Value.(int32))
		case int64:
			value = fmt.Sprintf("%field", field.Value.(int64))
		}
		values.Add(field.ModelField.Id, value)
	}

	page := 0

	for {
		m.neaktor.apiLimiter.Take()

		response, err := m.neaktor.runner.GetJson(HttpRunner.JsonRequestData{
			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&%s&size=50&page=%d", m.modelId, values.Encode(), page),
			Value: nil,
			Headers: map[string]string{
				"Authorization": m.neaktor.token,
			},
		})
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&%s&size=50&page=%d response error: %w", m.modelId, values.Encode(), page, err)
		}

		var tasksResponse TasksResponse
		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
			return tasks, fmt.Errorf("unmarshaling error: %w", err)
		}
		if len(tasksResponse.Code) > 0 {
			return tasks, parseErrorCode(tasksResponse.Code, tasksResponse.Message)
		}

		for _, data := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			for _, field := range data.Fields {
				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			tasks = append(tasks, NewTask(m, data.Id, data.Idx, fields))
		}

		if tasksResponse.Total < 50 {
			break
		}

		if float64(page) >= math.Ceil(float64(tasksResponse.Total/50)) {
			break
		}

		page++
	}

	return tasks, nil
}

//import (
//	"encoding/json"
//	"fmt"
//	"time"
//
//	log "github.com/sirupsen/logrus"
//)
//
//type ModelField struct {
//	Id    string `json:"id"`
//	Name  string `json:"name"`
//	State string `json:"state"`
//}
//
//type ModelStatus struct {
//	Id     string `json:"id"`
//	Name   string `json:"name"`
//	Closed bool   `json:"closed"`
//	Type   string `json:"type"`
//}
//
//type ModelRoles struct {
//	Id   string `json:"id"`
//	Name string `json:"name"`
//}
//
//type TaskModelResponseData struct {
//	Id               string         `json:"id"`
//	Name             string         `json:"name"`
//	CreatedBy        int            `json:"createdBy"`
//	LastModifiedBy   *int           `json:"lastModifiedBy"`
//	CreatedDate      string         `json:"createdDate"`
//	LastModifiedDate *string        `json:"lastModifiedDate"`
//	Fields           []ModelField   `json:"fields"`
//	Statuses         []ModelStatus `json:"statuses"`
//	StartStatus      string         `json:"startStatus"`
//	CanCreateTask    bool           `json:"canCreateTask"`
//	ModuleId         string         `json:"moduleId"`
//	Roles            []ModelRoles    `json:"roles"`
//	DeadlineStatus   string         `json:"deadlineStatus"`
//}
//
//type TaskModelResponse struct {
//	Data  []TaskModelResponseData `json:"data"`
//	Page  int                     `json:"page"`
//	Size  int                     `json:"size"`
//	Total int                     `json:"total"`
//	NeaktorErrorResponse
//}
//
//func (n *Neaktor) GetModelInfo(title string) (modelId string, statuses []ModelStatus, fields []ModelField, err error) {
//	n.apiLimiter.Take()
//
//	response, err := n.runner.getJson(JsonRequestData{ // todo внутренний кеш так как могут спрашивать модели одни за другим
//		url:   "https://api.neaktor.com" + "/v1/taskmodels?size=100",
//		value: nil,
//		headers: map[string]string{
//			"Authorization": n.token,
//		},
//	})
//	if err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return modelId, statuses, fields, fmt.Errorf("/v1/taskmodels?size=100 response error: %w", err)
//	}
//
//	var taskModelResponse TaskModelResponse
//	if err := json.Unmarshal(response.Body(), &taskModelResponse); err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return modelId, statuses, fields, fmt.Errorf("unmarshal error: %w", err)
//	}
//	if len(taskModelResponse.Code) > 0 {
//		return modelId, statuses, fields, fmt.Errorf("server error")
//	}
//
//	n.modelLastUpdatedAt = time.Now()
//
//	for _, item := range taskModelResponse.Data {
//		if item.Name == title {
//			return item.Id, item.Statuses, item.Fields, nil
//		}
//	}
//
//	return modelId, statuses, fields, fmt.Errorf("model title not found")
//}
//
//type RoutingsResponseAssingees struct {
//	Id   int    `json:"id"`
//	Name string `json:"name"`
//	Type string `json:"type"`
//}
//
//type RoutingsResponse struct {
//	To         string                      `json:"to"`
//	Conditions []interface{}               `json:"conditions"`
//	Assignees  []RoutingsResponseAssingees `json:"assignees"`
//}
//
//func (n *Neaktor) GetModelRoutingsByModel(modelId, statusId string) (userId int, err error) {
//	n.apiLimiter.Take()
//
//	response, err := n.runner.getJson(JsonRequestData{
//		url:   fmt.Sprintf("https://api.neaktor.com"+"/v1/taskmodels/%s/%s/routings", modelId, statusId),
//		value: nil,
//		headers: map[string]string{
//			"Authorization": n.token,
//		},
//	})
//	if err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return userId, fmt.Errorf("/v1/taskmodels/%s/%s/routings response error: %w", modelId, statusId, err)
//	}
//
//	var routingsResponses []RoutingsResponse
//	if err := json.Unmarshal(response.Body(), &routingsResponses); err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return userId, fmt.Errorf("unmarshal error: %w", err)
//	}
//	//if len(routingsResponses.Type) > 0 {
//	//	return userId, fmt.Errorf("server error")
//	//}
//
//	for _, routing := range routingsResponses {
//		if routing.To == statusId {
//			var assignees []int
//			for _, assignee := range routing.Assignees {
//				assignees = append(assignees, assignee.Id)
//			}
//
//			return randomFromIntSlice(assignees), nil
//		}
//	}
//
//	return userId, fmt.Errorf("assignee user id not found")
//}

//type TaskDataField struct {
//	Id    string      `json:"id"`
//	Value interface{} `json:"value"`
//	State string      `json:"state"`
//}
//
//type TasksResponseData struct {
//	Id         int              `json:"id"`
//	ProjectId  string           `json:"projectId"`
//	Fields     []TaskDataField `json:"fields"`
//	Status     string           `json:"status"`
//	ModelId    string           `json:"modelId"`
//	CanDelete  bool             `json:"canDelete"`
//	ModuleId   string           `json:"moduleId"`
//	Idx        string           `json:"idx"`
//	ParentId   interface{}      `json:"parentId"`
//	SubtaskIds []interface{}    `json:"subtaskIds"`
//}
//
//type TasksResponseLinks struct {
//	Next string `json:"next"`
//}
//
//type TasksResponse struct {
//	NeaktorErrorResponse
//	Data  []TasksResponseData `json:"data"`
//	Links TasksResponseLinks  `json:"links"`
//	Page  int                 `json:"page"`
//	Size  int                 `json:"size"`
//	Total int                 `json:"total"`
//}
//
//func (n *Neaktor) GetModelTasksByStatus(modelId, statusId string) (tasks []TasksResponseData, err error) {
//	limit := 50
//	maxPages := 1
//
//	for page := 0; page < maxPages; page++ {
//		n.apiLimiter.Take()
//
//		response, err := n.runner.GetJson(HttpRunner.JsonRequestData{
//			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d", modelId, statusId, limit, page),
//			Value: nil,
//			Headers: map[string]string{
//				"Authorization": n.token,
//			},
//		})
//		if err != nil {
//			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d response error: %w", modelId, statusId, limit, page, err)
//		}
//
//		var tasksResponse TasksResponse
//		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
//			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//			return tasks, fmt.Errorf("unmarshal error: %w", err)
//		}
//		if len(tasksResponse.Code) > 0 {
//			return tasks, fmt.Errorf("server error: %s", tasksResponse.Error)
//		}
//
//		tasks = append(tasks, tasksResponse.Data...)
//
//		//
//
//		maxPages = int(math.Ceil(float64(tasksResponse.Total) / float64(limit)))
//	}
//
//	return tasks, nil
//}
//
//func (n *Neaktor) GetModelTasksByStatuses(modelId string, statusIds []string) (tasks []TasksResponseData, err error) {
//	for _, statusId := range statusIds {
//		tasksByStatus, err := n.GetModelTasksByStatus(modelId, statusId)
//		if err != nil {
//			return tasks, err
//		}
//
//		tasks = append(tasks, tasksByStatus...)
//	}
//
//	return tasks, nil
//}
//
//func (n *Neaktor) GetModelTasksByStatusAndFields(modelId, statusId string, dataFields []TaskDataField) (tasks []TasksResponseData, err error) {
//	values := url.Values{}
//	for _, d := range dataFields {
//		var value string
//		switch d.Value.(type) {
//		case string:
//			value = d.Value.(string)
//		case float64:
//			value = fmt.Sprintf("%f", d.Value.(float64))
//		case float32:
//			value = fmt.Sprintf("%f", d.Value.(float32))
//		case int:
//			value = fmt.Sprintf("%d", d.Value.(int))
//		case int8:
//			value = fmt.Sprintf("%d", d.Value.(int8))
//		case int16:
//			value = fmt.Sprintf("%d", d.Value.(int16))
//		case int32:
//			value = fmt.Sprintf("%d", d.Value.(int32))
//		case int64:
//			value = fmt.Sprintf("%d", d.Value.(int64))
//		}
//		values.Add(d.Id, value)
//	}
//
//	page := 0
//
//	for {
//		n.apiLimiter.Take()
//
//		response, err := n.runner.GetJson(HttpRunner.JsonRequestData{
//			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&%s&size=50&page=%d", modelId, statusId, values.Encode(), page),
//			Value: nil,
//			Headers: map[string]string{
//				"Authorization": n.token,
//			},
//		})
//		if err != nil {
//			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&%s&size=%d response error: %w", modelId, statusId, values.Encode(), page, err)
//		}
//
//		var tasksResponse TasksResponse
//		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
//			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//			return tasks, fmt.Errorf("unmarshal error: %w", err)
//		}
//		if len(tasksResponse.Code) > 0 {
//			return tasks, fmt.Errorf("server error: %s", tasksResponse.Error)
//		}
//
//		tasks = append(tasks, tasksResponse.Data...)
//
//		if tasksResponse.Total < 50 {
//			break
//		}
//
//		if float64(page) >= math.Ceil(float64(tasksResponse.Total/50)) {
//			break
//		}
//
//		page++
//	}
//
//	return tasks, nil
//}
//
//func (n *Neaktor) GetModelTasksByFields(modelId string, dataFields []TaskDataField) (tasks []TasksResponseData, err error) {
//	values := url.Values{}
//	for _, d := range dataFields {
//		var value string
//		switch d.Value.(type) {
//		case string:
//			value = d.Value.(string)
//		case float64:
//			value = fmt.Sprintf("%f", d.Value.(float64))
//		case float32:
//			value = fmt.Sprintf("%f", d.Value.(float32))
//		case int:
//			value = fmt.Sprintf("%d", d.Value.(int))
//		case int8:
//			value = fmt.Sprintf("%d", d.Value.(int8))
//		case int16:
//			value = fmt.Sprintf("%d", d.Value.(int16))
//		case int32:
//			value = fmt.Sprintf("%d", d.Value.(int32))
//		case int64:
//			value = fmt.Sprintf("%d", d.Value.(int64))
//		}
//		values.Add(d.Id, value)
//	}
//
//	page := 0
//
//	for {
//		n.apiLimiter.Take()
//
//		response, err := n.runner.GetJson(HttpRunner.JsonRequestData{
//			Url:   fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&%s&size=50&page=%d", modelId, values.Encode(), page),
//			Value: nil,
//			Headers: map[string]string{
//				"Authorization": n.token,
//			},
//		})
//		if err != nil {
//			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&%s&size=50&page=%d response error: %w", modelId, values.Encode(), page, err)
//		}
//
//		var tasksResponse TasksResponse
//		if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
//			log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//			return tasks, fmt.Errorf("unmarshal error: %w", err)
//		}
//		if len(tasksResponse.Code) > 0 {
//			return tasks, fmt.Errorf("server error: %s", tasksResponse.Error)
//		}
//
//		tasks = append(tasks, tasksResponse.Data...)
//
//		if tasksResponse.Total < 50 {
//			break
//		}
//
//		if float64(page) >= math.Ceil(float64(tasksResponse.Total/50)) {
//			break
//		}
//
//		page++
//	}
//
//	return tasks, nil
//}
