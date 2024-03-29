package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wangluozhe/requests"
	requrl "github.com/wangluozhe/requests/url"
	"strconv"
	"time"
)

type CurrencyFieldValue struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type TaskField struct {
	ModelField ModelField
	Value      interface{}
	State      string
}

type Task struct {
	model            *Model
	status           ModelStatus
	id               int
	idx              string
	startDate        time.Time
	endDate          time.Time
	statusClosedDate time.Time
	fields           []TaskField
}

var ErrTaskNotFound = errors.New("TASK_NOT_FOUND")
var ErrTaskFieldNotFound = errors.New("TASK_FIELD_NOT_FOUND")

type ITask interface {
	GetId() int
	GetIdx() string
	GetStartDate() time.Time
	GetEndDate() time.Time
	GetStatusClosedDate() time.Time
	GetStatus() ModelStatus
	GetField(modelField ModelField) (taskField TaskField, err error)
	MustGetField(modelField ModelField) (taskField TaskField)
	GetCustomField(modelField ModelField) (taskField TaskField, err error)
	MustGetCustomField(modelField ModelField) (taskField TaskField)
	UpdateFields(fields []TaskField) error
	MustUpdateFields(fields []TaskField)
	UpdateStatus(status ModelStatus) error
	MustUpdateStatus(status ModelStatus)
	AddComment(message string) error
	MustAddComment(message string)
}

func NewTask(model *Model, status ModelStatus, id int, idx string, startDate, endDate, statusClosedDate time.Time, fields []TaskField) ITask {
	return &Task{
		model:            model,
		status:           status,
		id:               id,
		idx:              idx,
		startDate:        startDate,
		endDate:          endDate,
		statusClosedDate: statusClosedDate,
		fields:           fields,
	}
}

func (t *Task) GetId() int {
	return t.id
}

func (t *Task) GetIdx() string {
	return t.idx
}

func (t *Task) GetStartDate() time.Time {
	return t.startDate
}

func (t *Task) GetEndDate() time.Time {
	return t.endDate
}

func (t *Task) GetStatusClosedDate() time.Time {
	return t.statusClosedDate
}

func (t *Task) GetStatus() ModelStatus {
	return t.status
}

func (t *Task) GetField(modelField ModelField) (taskField TaskField, err error) {
	for _, field := range t.fields {
		if field.ModelField.Id == modelField.Id {
			return field, err
		}
	}

	return taskField, ErrTaskFieldNotFound
}

func (t *Task) MustGetField(modelField ModelField) (taskField TaskField) {
	var err error
	taskField, err = t.GetField(modelField)
	if err != nil {
		panic(err)
	}

	return taskField
}

func (t *Task) GetCustomField(modelField ModelField) (taskField TaskField, err error) {
	for _, field := range t.fields {
		if field.ModelField.Id == modelField.Id {
			value, err := t.model.GetCustomFieldValue(modelField, field.Value.(string))
			if err != nil {
				return field, err
			}

			field.Value = value
			return field, err
		}
	}

	return taskField, ErrTaskFieldNotFound
}

func (t *Task) MustGetCustomField(modelField ModelField) (taskField TaskField) {
	var err error
	taskField, err = t.GetCustomField(modelField)
	if err != nil {
		panic(err)
	}

	return taskField
}

func (t *Task) UpdateFields(fields []TaskField) error {
	type UpdateTaskRequestAssignee struct {
		Id   int    `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
	}

	type UpdateTaskRequestField struct {
		Id    string      `json:"id,omitempty"`
		Value interface{} `json:"value,omitempty"`
	}

	type UpdateTaskRequest struct {
		StartDate string                     `json:"startDate,omitempty"`
		EndDate   string                     `json:"endDate,omitempty"`
		Assignee  *UpdateTaskRequestAssignee `json:"assignee,omitempty"`
		Fields    []UpdateTaskRequestField   `json:"fields,omitempty"`
	}

	type UpdateTasksResponse struct {
		NeaktorErrorResponse
	}

	//

	t.model.neaktor.apiLimiter.Take()

	updateFields := make([]UpdateTaskRequestField, 0)

	for _, field := range fields {
		updateFields = append(updateFields, UpdateTaskRequestField{
			Id:    field.ModelField.Id,
			Value: field.Value,
		})
	}

	updateTasksRequest := UpdateTaskRequest{
		Fields: updateFields,
	}
	updateTasksRequestBytes, err := json.Marshal(updateTasksRequest)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	httpClient := t.model.neaktor.httpClient

	httpClient.Headers = requrl.NewHeaders()
	httpClient.Headers.Add("Authorization", t.model.neaktor.token)

	httpClient.Body = string(updateTasksRequestBytes)

	response, err := requests.Put(mustUrlJoinPath(ApiGateway, "tasks", strconv.Itoa(t.id)), &httpClient)
	if err != nil {
		return fmt.Errorf("/v1/tasks/%d request error: %w", t.id, err)
	}

	if response.StatusCode >= 500 {
		t.model.neaktor.log.Debugf("response status code: %d", response.StatusCode)
		return fmt.Errorf("service unavailable, code: %d", response.StatusCode)
	}

	var updateTasksResponse UpdateTasksResponse
	if err := json.Unmarshal(response.Content, &updateTasksResponse); err != nil {
		t.model.neaktor.log.Debugf("response code: %d, response body: %v", response.StatusCode, response.Text)
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(updateTasksResponse.Code) > 0 {
		return parseErrorCode(updateTasksResponse.Code, updateTasksResponse.Message)
	}

	return err
}

func (t *Task) MustUpdateFields(fields []TaskField) {
	var err error
	if err = t.UpdateFields(fields); err != nil {
		panic(err)
	}
}

func (t *Task) UpdateStatus(status ModelStatus) error {
	type UpdateTaskStatusRequestAssignee struct {
		Id   int    `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
	}

	type UpdateTaskStatusRequest struct {
		Status      string                           `json:"status,omitempty"`
		ConditionId string                           `json:"conditionId,omitempty"`
		Assignee    *UpdateTaskStatusRequestAssignee `json:"assignee,omitempty"`
	}

	type UpdateTaskStatusResponse struct {
		NeaktorErrorResponse
	}

	//

	t.model.neaktor.apiLimiter.Take()

	updateTaskStatusRequest := UpdateTaskStatusRequest{
		Status: status.Id,
	}
	updateTaskStatusRequestBytes, err := json.Marshal(updateTaskStatusRequest)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	httpClient := t.model.neaktor.httpClient

	httpClient.Headers = requrl.NewHeaders()
	httpClient.Headers.Add("Authorization", t.model.neaktor.token)

	httpClient.Body = string(updateTaskStatusRequestBytes)

	response, err := requests.Post(mustUrlJoinPath(ApiGateway, "tasks", strconv.Itoa(t.id), "status", "change"), &httpClient)
	if err != nil {
		return fmt.Errorf("/v1/tasks/%d/status/change request error: %w", t.id, err)
	}

	if response.StatusCode >= 500 {
		t.model.neaktor.log.Debugf("response status code: %d", response.StatusCode)
		return fmt.Errorf("service unavailable, code: %d", response.StatusCode)
	}

	var updateTaskStatusResponse UpdateTaskStatusResponse
	if err := json.Unmarshal(response.Content, &updateTaskStatusResponse); err != nil {
		t.model.neaktor.log.Debugf("response code: %d, response body: %v", response.StatusCode, response.Text)
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(updateTaskStatusResponse.Code) > 0 {
		return parseErrorCode(updateTaskStatusResponse.Code, updateTaskStatusResponse.Message)
	}

	return err
}

func (t *Task) MustUpdateStatus(status ModelStatus) {
	var err error
	if err = t.UpdateStatus(status); err != nil {
		panic(err)
	}
}

func (t *Task) AddComment(message string) error {
	type CreateCommentToTaskRequest struct {
		Text string `json:"text"`
	}

	type CreateCommentToTaskResponse struct {
		NeaktorErrorResponse
	}

	//

	createCommentToTaskRequest := CreateCommentToTaskRequest{
		Text: message,
	}
	createCommentToTaskRequestBytes, err := json.Marshal(createCommentToTaskRequest)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	httpClient := t.model.neaktor.httpClient

	httpClient.Headers = requrl.NewHeaders()
	httpClient.Headers.Add("Authorization", t.model.neaktor.token)

	httpClient.Body = string(createCommentToTaskRequestBytes)

	response, err := requests.Post(mustUrlJoinPath(ApiGateway, "comments", strconv.Itoa(t.id)), &httpClient)
	if err != nil {
		return fmt.Errorf("/v1/comments/%d request error: %w", t.id, err)
	}

	if response.StatusCode >= 500 {
		t.model.neaktor.log.Debugf("response status code: %d", response.StatusCode)
		return fmt.Errorf("service unavailable, code: %d", response.StatusCode)
	}

	var createCommentToTaskResponse CreateCommentToTaskResponse
	if err := json.Unmarshal(response.Content, &createCommentToTaskResponse); err != nil {
		t.model.neaktor.log.Debugf("response code: %d, response body: %v", response.StatusCode, response.Text)
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(createCommentToTaskResponse.Code) > 0 {
		return parseErrorCode(createCommentToTaskResponse.Code, createCommentToTaskResponse.Message)
	}

	return err
}

func (t *Task) MustAddComment(message string) {
	var err error
	if err = t.AddComment(message); err != nil {
		panic(err)
	}
}
