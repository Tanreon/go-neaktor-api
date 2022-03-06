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
	id       string
	statuses map[string]ModelStatus
	fields   map[string]ModelField
}

var ErrModelStatusNotFound = errors.New("MODEL_STATUS_NOT_FOUND")
var ErrModelFieldNotFound = errors.New("MODEL_FIELD_NOT_FOUND")

type IModel interface {
	GetId() string
	GetAllStatuses() (statuses map[string]ModelStatus)
	GetAllFields() (fields map[string]ModelField)
	GetStatuses(titles []string) (statuses map[string]ModelStatus, err error)
	GetFields(titles []string) (fields map[string]ModelField, err error)
	GetStatus(title string) (status ModelStatus, err error)
	GetField(title string) (field ModelField, err error)
	GetTasksByStatus(status ModelStatus) (tasks []ITask, err error)
	GetTasksByStatuses(statuses []ModelStatus) (tasks []ITask, err error)
	GetTasksByStatusAndFields(status ModelStatus, fields []TaskField) (tasks []ITask, err error)
	GetTasksByFields(fields []TaskField) (tasks []ITask, err error)
	GetTaskById(id int) (task ITask, err error)
	IsTasksByStatusExists(status ModelStatus) (isExists bool, err error)
	IsTasksByStatusesExists(statuses []ModelStatus) (isExists bool, err error)
	IsTasksByStatusAndFieldsExists(status ModelStatus, fields []TaskField) (isExists bool, err error)
	IsTasksByFieldsExists(fields []TaskField) (isExists bool, err error)
}

func NewModel(neaktor *Neaktor, id string, statuses map[string]ModelStatus, fields map[string]ModelField) IModel {
	return &Model{
		neaktor:  neaktor,
		id:       id,
		statuses: statuses,
		fields:   fields,
	}
}

func (m *Model) GetId() string {
	return m.id
}

func (m *Model) GetAllStatuses() (statuses map[string]ModelStatus) {
	return m.statuses
}

func (m *Model) GetAllFields() (fields map[string]ModelField) {
	return m.fields
}

func (m *Model) GetStatuses(titles []string) (statuses map[string]ModelStatus, err error) {
	statuses = make(map[string]ModelStatus, 0)

	for _, modelStatus := range m.statuses {
		for _, title := range titles {
			if strings.EqualFold(modelStatus.Name, title) {
				statuses[title] = modelStatus
			}
		}
	}

	if len(statuses) <= 0 {
		return statuses, ErrModelStatusNotFound
	}

	return statuses, err
}

func (m *Model) GetFields(titles []string) (fields map[string]ModelField, err error) {
	fields = make(map[string]ModelField, 0)

	for _, modelField := range m.fields {
		for _, title := range titles {
			if strings.EqualFold(modelField.Name, title) {
				fields[title] = modelField
			}
		}
	}

	if len(fields) <= 0 {
		return fields, ErrModelFieldNotFound
	}

	return fields, err
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

//

func (m *Model) IsTasksByStatusExists(status ModelStatus) (isExists bool, err error) {
	tasks, err := m.GetTasksByStatus(status)
	if err != nil {
		return isExists, err
	}

	return len(tasks) > 0, err
}

func (m *Model) IsTasksByStatusesExists(statuses []ModelStatus) (isExists bool, err error) {
	tasks, err := m.GetTasksByStatuses(statuses)
	if err != nil {
		return isExists, err
	}

	return len(tasks) > 0, err
}

func (m *Model) IsTasksByStatusAndFieldsExists(status ModelStatus, fields []TaskField) (isExists bool, err error) {
	tasks, err := m.GetTasksByStatusAndFields(status, fields)
	if err != nil {
		return isExists, err
	}

	return len(tasks) > 0, err
}

func (m *Model) IsTasksByFieldsExists(fields []TaskField) (isExists bool, err error) {
	tasks, err := m.GetTasksByFields(fields)
	if err != nil {
		return isExists, err
	}

	return len(tasks) > 0, err
}

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

		jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d", m.id, status.Id, limit, page))
		jsonRequestData.SetHeaders(map[string]string{
			"Authorization": m.neaktor.token,
		})

		response, err := m.neaktor.runner.GetJson(jsonRequestData)
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&size=%d&page=%d response error: %w", m.id, status.Id, limit, page, err)
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

func (m *Model) GetTasksByStatusAndFields(status ModelStatus, fields []TaskField) (tasks []ITask, err error) {
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
	for _, field := range fields {
		var value string
		switch field.Value.(type) {
		case string:
			value = field.Value.(string)
		case float64:
			value = fmt.Sprintf("%f", field.Value.(float64))
		case float32:
			value = fmt.Sprintf("%f", field.Value.(float32))
		case int:
			value = fmt.Sprintf("%d", field.Value.(int))
		case int8:
			value = fmt.Sprintf("%d", field.Value.(int8))
		case int16:
			value = fmt.Sprintf("%d", field.Value.(int16))
		case int32:
			value = fmt.Sprintf("%d", field.Value.(int32))
		case int64:
			value = fmt.Sprintf("%d", field.Value.(int64))
		}
		values.Add(field.ModelField.Id, value)
	}

	page := 0

	for {
		m.neaktor.apiLimiter.Take()

		jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&status_id=%s&%s&size=50&page=%d", m.id, status.Id, values.Encode(), page))
		jsonRequestData.SetHeaders(map[string]string{
			"Authorization": m.neaktor.token,
		})

		response, err := m.neaktor.runner.GetJson(jsonRequestData)
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&status_id=%s&%s&size=%d response error: %w", m.id, status.Id, values.Encode(), page, err)
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

func (m *Model) GetTasksByFields(fields []TaskField) (tasks []ITask, err error) {
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
	for _, field := range fields {
		var value string
		switch field.Value.(type) {
		case string:
			value = field.Value.(string)
		case float64:
			value = fmt.Sprintf("%f", field.Value.(float64))
		case float32:
			value = fmt.Sprintf("%f", field.Value.(float32))
		case int:
			value = fmt.Sprintf("%d", field.Value.(int))
		case int8:
			value = fmt.Sprintf("%d", field.Value.(int8))
		case int16:
			value = fmt.Sprintf("%d", field.Value.(int16))
		case int32:
			value = fmt.Sprintf("%d", field.Value.(int32))
		case int64:
			value = fmt.Sprintf("%d", field.Value.(int64))
		}
		values.Add(field.ModelField.Id, value)
	}

	page := 0

	for {
		m.neaktor.apiLimiter.Take()

		jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/tasks?model_id=%s&%s&size=50&page=%d", m.id, values.Encode(), page))
		jsonRequestData.SetHeaders(map[string]string{
			"Authorization": m.neaktor.token,
		})

		response, err := m.neaktor.runner.GetJson(jsonRequestData)
		if err != nil {
			return tasks, fmt.Errorf("/v1/tasks?model_id=%s&%s&size=50&page=%d response error: %w", m.id, values.Encode(), page, err)
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

func (m *Model) GetTaskById(id int) (task ITask, err error) {
	type TaskDataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TaskResponse struct {
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

	//

	m.neaktor.apiLimiter.Take()

	jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/tasks/%d", id))
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": m.neaktor.token,
	})

	response, err := m.neaktor.runner.GetJson(jsonRequestData)
	if err != nil {
		return task, fmt.Errorf("/v1/tasks/%d response error: %w", id, err)
	}

	var tasksResponse []TaskResponse
	if err := json.Unmarshal(response.Body(), &tasksResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return task, fmt.Errorf("unmarshaling error: %w", err)
	}
	//if len(tasksResponse.Code) > 0 {
	//	return task, parseErrorCode(tasksResponse.Code, tasksResponse.Message)
	//}

	for _, taskData := range tasksResponse {
		fields := make([]TaskField, 0)

		for _, field := range taskData.Fields {
			fields = append(fields, TaskField{
				ModelField: m.fields[field.Id],
				Value:      field.Value,
				State:      field.State,
			})
		}

		return NewTask(m, taskData.Id, taskData.Idx, fields), err
	}

	return task, ErrTaskNotFound
}
