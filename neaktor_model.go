package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

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

type CustomFieldOption struct {
	id    string
	value string
}
type ModelCustomFieldCache struct {
	lastUpdatedAt      time.Time
	customFieldOptions []CustomFieldOption
}

type ModelAssignee struct {
	id     int
	name   string
	typeOf string
}
type ModelAssigneeCache struct {
	lastUpdatedAt  time.Time
	modelAssignees []ModelAssignee
}

type Model struct {
	neaktor  *Neaktor
	id       string
	statuses map[string]ModelStatus
	fields   map[string]ModelField

	modelCustomFieldCacheLock sync.Mutex
	modelCustomFieldCacheMap  map[string]ModelCustomFieldCache

	modelAssigneeCacheLock sync.Mutex
	modelAssigneeCacheMap  map[string]ModelAssigneeCache
}

var ErrModelStatusNotFound = errors.New("MODEL_STATUS_NOT_FOUND")
var ErrModelFieldNotFound = errors.New("MODEL_FIELD_NOT_FOUND")
var ErrModelCustomFieldOptionNotFound = errors.New("MODEL_CUSTOM_FIELD_OPTION_NOT_FOUND")
var ErrModelCustomFieldValueNotFound = errors.New("MODEL_CUSTOM_FIELD_VALUE_NOT_FOUND")
var ErrModelAssigneeNotFound = errors.New("MODEL_ASSIGNEE_NOT_FOUND")

type IModel interface {
	GetId() string
	GetAllStatuses() (statuses map[string]ModelStatus)
	GetAllFields() (fields map[string]ModelField)
	GetStatuses(titles []string) (statuses map[string]ModelStatus, err error)
	GetFields(titles []string) (fields map[string]ModelField, err error)
	GetStatus(title string) (status ModelStatus, err error)
	GetField(title string) (field ModelField, err error)
	GetCustomFieldOptionId(field ModelField, value string) (optionId string, err error)
	GetCustomFieldValue(field ModelField, optionId string) (value string, err error)
	GetAssignee(status ModelStatus, name string) (assignee ModelAssignee, err error)
	GetTasksByStatus(status ModelStatus) (tasks []ITask, err error)
	GetTasksByStatuses(statuses []ModelStatus) (tasks []ITask, err error)
	GetTasksByStatusAndFields(status ModelStatus, fields []TaskField) (tasks []ITask, err error)
	GetTasksByFields(fields []TaskField) (tasks []ITask, err error)
	GetTaskById(id int) (task ITask, err error)
	IsTasksByStatusExists(status ModelStatus) (isExists bool, err error)
	IsTasksByStatusesExists(statuses []ModelStatus) (isExists bool, err error)
	IsTasksByStatusAndFieldsExists(status ModelStatus, fields []TaskField) (isExists bool, err error)
	IsTasksByFieldsExists(fields []TaskField) (isExists bool, err error)
	CreateTask(assignee ModelAssignee, fields []TaskField) (task ITask, err error)
}

func NewModel(neaktor *Neaktor, id string, statuses map[string]ModelStatus, fields map[string]ModelField) IModel {
	return &Model{
		neaktor:                   neaktor,
		id:                        id,
		statuses:                  statuses,
		fields:                    fields,
		modelCustomFieldCacheLock: sync.Mutex{},
		modelCustomFieldCacheMap:  make(map[string]ModelCustomFieldCache, 0),
		modelAssigneeCacheLock:    sync.Mutex{},
		modelAssigneeCacheMap:     make(map[string]ModelAssigneeCache, 0),
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

func (m *Model) GetCustomFieldOptionId(field ModelField, value string) (optionId string, err error) {
	type OptionsAvailableValues struct {
		Id    string `json:"id"`
		Value string `json:"value"`
	}

	type CustomFieldsResponseOptions struct {
		AvailableValues []OptionsAvailableValues `json:"availableValues"`
	}

	type CustomFieldsResponse struct {
		NeaktorErrorResponse
		Id      string                      `json:"id"`
		Type    string                      `json:"type"`
		Name    string                      `json:"name"`
		Options CustomFieldsResponseOptions `json:"options"`
	}

	m.modelCustomFieldCacheLock.Lock()
	defer m.modelCustomFieldCacheLock.Unlock()

	// cache first

	if cachedModelCustomField, present := m.modelCustomFieldCacheMap[field.Id]; present {
		if time.Now().Before(cachedModelCustomField.lastUpdatedAt.Add(MODEL_CACHE_TIME)) {
			for _, customFieldOption := range cachedModelCustomField.customFieldOptions {
				if customFieldOption.value == value {
					return customFieldOption.id, err
				}
			}
		}

		delete(m.modelCustomFieldCacheMap, field.Id)
	}

	// request second

	jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/customfields/%s", field.Id))
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": m.neaktor.token,
	})

	response, err := m.neaktor.runner.GetJson(jsonRequestData)
	if err != nil {
		return optionId, fmt.Errorf("/v1/customfields/%s response error: %w", field.Id, err)
	}

	var customFieldsResponses []CustomFieldsResponse
	if err := json.Unmarshal(response.Body(), &customFieldsResponses); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return optionId, fmt.Errorf("unmarshaling error: %w", err)
	}
	//if len(createTaskResponse.Code) > 0 {
	//	return task, parseErrorCode(createTaskResponse.Code, createTaskResponse.Message)
	//}

	for _, cutomField := range customFieldsResponses {
		customFieldOptions := make([]CustomFieldOption, 0)

		for _, item := range cutomField.Options.AvailableValues {
			customFieldOptions = append(customFieldOptions, CustomFieldOption{
				id:    item.Id,
				value: item.Value,
			})
		}

		m.modelCustomFieldCacheMap[field.Id] = ModelCustomFieldCache{
			lastUpdatedAt:      time.Now(),
			customFieldOptions: customFieldOptions,
		}
	}

	//

	if cachedModelCustomField, present := m.modelCustomFieldCacheMap[field.Id]; present {
		for _, customFieldOption := range cachedModelCustomField.customFieldOptions {
			if customFieldOption.value == value {
				return customFieldOption.id, err
			}
		}
	}

	return optionId, ErrModelCustomFieldOptionNotFound
}

func (m *Model) GetCustomFieldValue(field ModelField, optionId string) (value string, err error) {
	type OptionsAvailableValues struct {
		Id    string `json:"id"`
		Value string `json:"value"`
	}

	type CustomFieldsResponseOptions struct {
		AvailableValues []OptionsAvailableValues `json:"availableValues"`
	}

	type CustomFieldsResponse struct {
		NeaktorErrorResponse
		Id      string                      `json:"id"`
		Type    string                      `json:"type"`
		Name    string                      `json:"name"`
		Options CustomFieldsResponseOptions `json:"options"`
	}

	m.modelCustomFieldCacheLock.Lock()
	defer m.modelCustomFieldCacheLock.Unlock()

	// cache first

	if cachedModelCustomField, present := m.modelCustomFieldCacheMap[field.Id]; present {
		if time.Now().Before(cachedModelCustomField.lastUpdatedAt.Add(MODEL_CACHE_TIME)) {
			for _, customFieldOption := range cachedModelCustomField.customFieldOptions {
				if customFieldOption.id == optionId {
					return customFieldOption.value, err
				}
			}
		}

		delete(m.modelCustomFieldCacheMap, field.Id)
	}

	// request second

	jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/customfields/%s", field.Id))
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": m.neaktor.token,
	})

	response, err := m.neaktor.runner.GetJson(jsonRequestData)
	if err != nil {
		return value, fmt.Errorf("/v1/customfields/%s response error: %w", field.Id, err)
	}

	var customFieldsResponses []CustomFieldsResponse
	if err := json.Unmarshal(response.Body(), &customFieldsResponses); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return value, fmt.Errorf("unmarshaling error: %w", err)
	}
	//if len(createTaskResponse.Code) > 0 {
	//	return task, parseErrorCode(createTaskResponse.Code, createTaskResponse.Message)
	//}

	for _, cutomField := range customFieldsResponses {
		customFieldOptions := make([]CustomFieldOption, 0)

		for _, item := range cutomField.Options.AvailableValues {
			customFieldOptions = append(customFieldOptions, CustomFieldOption{
				id:    item.Id,
				value: item.Value,
			})
		}

		m.modelCustomFieldCacheMap[field.Id] = ModelCustomFieldCache{
			lastUpdatedAt:      time.Now(),
			customFieldOptions: customFieldOptions,
		}
	}

	//

	if cachedModelCustomField, present := m.modelCustomFieldCacheMap[field.Id]; present {
		for _, customFieldOption := range cachedModelCustomField.customFieldOptions {
			if customFieldOption.id == optionId {
				return customFieldOption.value, err
			}
		}
	}

	return value, ErrModelCustomFieldValueNotFound
}

func (m *Model) GetAssignee(status ModelStatus, name string) (assignee ModelAssignee, err error) {
	type RoutingResponseAssignee struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	type RoutingResponse struct {
		NeaktorErrorResponse
		To         string                    `json:"to"`
		Conditions []interface{}             `json:"conditions"`
		Assignees  []RoutingResponseAssignee `json:"assignees"`
	}

	m.modelAssigneeCacheLock.Lock()
	defer m.modelAssigneeCacheLock.Unlock()

	// cache first

	if cachedModelAssignee, present := m.modelAssigneeCacheMap[status.Id]; present {
		if time.Now().Before(cachedModelAssignee.lastUpdatedAt.Add(MODEL_CACHE_TIME)) {
			for _, modelAssignee := range cachedModelAssignee.modelAssignees {
				if modelAssignee.name == name {
					return modelAssignee, err
				}
			}
		}

		delete(m.modelAssigneeCacheMap, status.Id)
	}

	// request second

	jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/taskmodels/%s/%s/routings", m.id, status.Id))
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": m.neaktor.token,
	})

	response, err := m.neaktor.runner.GetJson(jsonRequestData)
	if err != nil {
		return assignee, fmt.Errorf("/v1/taskmodels/%s/%s/routings response error: %w", m.id, status.Id, err)
	}

	var routingResponses []RoutingResponse
	if err := json.Unmarshal(response.Body(), &routingResponses); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return assignee, fmt.Errorf("unmarshaling error: %w", err)
	}
	//if len(createTaskResponse.Code) > 0 {
	//	return task, parseErrorCode(createTaskResponse.Code, createTaskResponse.Message)
	//}

	for _, routing := range routingResponses {
		modelAssignees := make([]ModelAssignee, 0)

		for _, item := range routing.Assignees {
			modelAssignees = append(modelAssignees, ModelAssignee{
				id:     item.Id,
				name:   item.Name,
				typeOf: item.Type,
			})
		}

		m.modelAssigneeCacheMap[routing.To] = ModelAssigneeCache{
			lastUpdatedAt:  time.Now(),
			modelAssignees: modelAssignees,
		}
	}

	//

	if cachedModel, present := m.modelAssigneeCacheMap[status.Id]; present {
		for _, modelAssignee := range cachedModel.modelAssignees {
			if modelAssignee.name == name {
				return modelAssignee, err
			}
		}
	}

	return assignee, ErrModelAssigneeNotFound
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
	type DataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int           `json:"id"`
		ProjectId  string        `json:"projectId"`
		Fields     []DataField   `json:"fields"`
		Status     string        `json:"status"`
		ModelId    string        `json:"modelId"`
		CanDelete  bool          `json:"canDelete"`
		ModuleId   string        `json:"moduleId"`
		Idx        string        `json:"idx"`
		ParentId   interface{}   `json:"parentId"`
		SubtaskIds []interface{} `json:"subtaskIds"`
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

		for _, taskData := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			var startDate time.Time
			var endDate time.Time
			var statusClosedDate time.Time

			for _, field := range taskData.Fields {
				if strings.EqualFold(field.Id, "start") && field.Value != nil {
					startDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task start parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "end") && field.Value != nil {
					endDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task end parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "statusClosedDate") && field.Value != nil {
					statusClosedDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task status closed parse error: %w", err)
					}
				}

				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			var modelStatus ModelStatus

			for _, status := range m.statuses {
				if strings.EqualFold(status.Name, taskData.Status) {
					modelStatus = status
				}
			}

			tasks = append(tasks, NewTask(m, modelStatus, taskData.Id, taskData.Idx, startDate, endDate, statusClosedDate, fields))
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
	type DataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int           `json:"id"`
		ProjectId  string        `json:"projectId"`
		Fields     []DataField   `json:"fields"`
		Status     string        `json:"status"`
		ModelId    string        `json:"modelId"`
		CanDelete  bool          `json:"canDelete"`
		ModuleId   string        `json:"moduleId"`
		Idx        string        `json:"idx"`
		ParentId   interface{}   `json:"parentId"`
		SubtaskIds []interface{} `json:"subtaskIds"`
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

		for _, taskData := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			var startDate time.Time
			var endDate time.Time
			var statusClosedDate time.Time

			for _, field := range taskData.Fields {
				if strings.EqualFold(field.Id, "start") && field.Value != nil {
					startDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task start parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "end") && field.Value != nil {
					endDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task end parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "statusClosedDate") && field.Value != nil {
					statusClosedDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task status closed parse error: %w", err)
					}
				}

				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			var modelStatus ModelStatus

			for _, status := range m.statuses {
				if strings.EqualFold(status.Name, taskData.Status) {
					modelStatus = status
				}
			}

			tasks = append(tasks, NewTask(m, modelStatus, taskData.Id, taskData.Idx, startDate, endDate, statusClosedDate, fields))
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
	type DataField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TasksResponseData struct {
		Id         int           `json:"id"`
		ProjectId  string        `json:"projectId"`
		Fields     []DataField   `json:"fields"`
		Status     string        `json:"status"`
		ModelId    string        `json:"modelId"`
		CanDelete  bool          `json:"canDelete"`
		ModuleId   string        `json:"moduleId"`
		Idx        string        `json:"idx"`
		ParentId   interface{}   `json:"parentId"`
		SubtaskIds []interface{} `json:"subtaskIds"`
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

		for _, taskData := range tasksResponse.Data {
			fields := make([]TaskField, 0)

			var startDate time.Time
			var endDate time.Time
			var statusClosedDate time.Time

			for _, field := range taskData.Fields {
				if strings.EqualFold(field.Id, "start") && field.Value != nil {
					startDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task start parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "end") && field.Value != nil {
					endDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task end parse error: %w", err)
					}
				}
				if strings.EqualFold(field.Id, "statusClosedDate") && field.Value != nil {
					statusClosedDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
					if err != nil {
						return tasks, fmt.Errorf("task status closed parse error: %w", err)
					}
				}

				fields = append(fields, TaskField{
					ModelField: m.fields[field.Id],
					Value:      field.Value,
					State:      field.State,
				})
			}

			var modelStatus ModelStatus

			for _, status := range m.statuses {
				if strings.EqualFold(status.Name, taskData.Status) {
					modelStatus = status
				}
			}

			tasks = append(tasks, NewTask(m, modelStatus, taskData.Id, taskData.Idx, startDate, endDate, statusClosedDate, fields))
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
	type TaskResponseField struct {
		Id    string      `json:"id"`
		Value interface{} `json:"value"`
		State string      `json:"state"`
	}

	type TaskResponse struct {
		Id         int                 `json:"id"`
		ProjectId  string              `json:"projectId"`
		Fields     []TaskResponseField `json:"fields"`
		Status     string              `json:"status"`
		ModelId    string              `json:"modelId"`
		CanDelete  bool                `json:"canDelete"`
		ModuleId   string              `json:"moduleId"`
		Idx        string              `json:"idx"`
		ParentId   interface{}         `json:"parentId"`
		SubtaskIds []interface{}       `json:"subtaskIds"`
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

		var startDate time.Time
		var endDate time.Time
		var statusClosedDate time.Time

		for _, field := range taskData.Fields {
			if strings.EqualFold(field.Id, "start") && field.Value != nil {
				startDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
				if err != nil {
					return task, fmt.Errorf("task start parse error: %w", err)
				}
			}
			if strings.EqualFold(field.Id, "end") && field.Value != nil {
				endDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
				if err != nil {
					return task, fmt.Errorf("task end parse error: %w", err)
				}
			}
			if strings.EqualFold(field.Id, "statusClosedDate") && field.Value != nil {
				statusClosedDate, err = time.Parse("02-01-2006T15:04:05", field.Value.(string))
				if err != nil {
					return task, fmt.Errorf("task status closed parse error: %w", err)
				}
			}

			fields = append(fields, TaskField{
				ModelField: m.fields[field.Id],
				Value:      field.Value,
				State:      field.State,
			})
		}

		var modelStatus ModelStatus

		for _, status := range m.statuses {
			if status.Id == taskData.Status {
				modelStatus = status
			}
		}

		return NewTask(m, modelStatus, taskData.Id, taskData.Idx, startDate, endDate, statusClosedDate, fields), err
	}

	return task, ErrTaskNotFound
}

func (m *Model) CreateTask(assignee ModelAssignee, fields []TaskField) (task ITask, err error) {
	type CreateTaskRequestAssignee struct {
		Id   int    `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
	}

	type CreateTaskRequestField struct {
		Id    string      `json:"id,omitempty"`
		Value interface{} `json:"value,omitempty"`
	}

	type CreateTaskRequest struct {
		Assignee CreateTaskRequestAssignee `json:"assignee"`
		Fields   []CreateTaskRequestField  `json:"fields"`
	}

	type CreateTaskResponse struct {
		NeaktorErrorResponse
		Id        int    `json:"id"`
		ProjectId string `json:"projectId"`
	}

	//

	m.neaktor.apiLimiter.Take()

	createFields := make([]CreateTaskRequestField, 0)

	for _, field := range fields {
		createFields = append(createFields, CreateTaskRequestField{
			Id:    field.ModelField.Id,
			Value: field.Value,
		})
	}

	createTaskReques := CreateTaskRequest{
		Fields: createFields,
		Assignee: CreateTaskRequestAssignee{
			Id:   assignee.id,
			Type: assignee.typeOf,
		},
	}
	createTaskRequestBytes, err := json.Marshal(createTaskReques)
	if err != nil {
		return task, fmt.Errorf("marshaling error: %w", err)
	}

	jsonRequestData := HttpRunner.NewJsonRequestData(fmt.Sprintf(API_SERVER+"/v1/tasks/%s", m.id))
	jsonRequestData.SetHeaders(map[string]string{
		"Authorization": m.neaktor.token,
	})
	jsonRequestData.SetValue(createTaskRequestBytes)

	response, err := m.neaktor.runner.PostJson(jsonRequestData)
	if err != nil {
		return task, fmt.Errorf("/v1/tasks/%s response error: %w", m.id, err)
	}

	var createTaskResponse CreateTaskResponse
	if err := json.Unmarshal(response.Body(), &createTaskResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return task, fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(createTaskResponse.Code) > 0 {
		return task, parseErrorCode(createTaskResponse.Code, createTaskResponse.Message)
	}

	//

	return m.GetTaskById(createTaskResponse.Id)
}
