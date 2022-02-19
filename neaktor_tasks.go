package neaktor_api

import (
	"encoding/json"
	"errors"
	"fmt"

	HttpRunner "github.com/Tanreon/go-http-runner"
	log "github.com/sirupsen/logrus"
)

type TaskField struct {
	ModelField ModelField
	Value      interface{}
	State      string
}

type Task struct {
	model  *Model
	id     int
	idx    string
	fields []TaskField
}

var ErrTaskNotFound = errors.New("TASK_NOT_FOUND")
var ErrTaskFieldNotFound = errors.New("TASK_FIELD_NOT_FOUND")

type ITask interface {
	GetId() int
	GetIdx() string
	GetField(modelField ModelField) (taskField TaskField, err error)
	UpdateFields([]TaskField) error
	UpdateStatus(status ModelStatus) error
	AddComment(message string) error
}

func NewTask(model *Model, id int, idx string, fields []TaskField) ITask {
	return &Task{
		model:  model,
		id:     id,
		idx:    idx,
		fields: fields,
	}
}

func (t *Task) GetId() int {
	return t.id
}

func (t *Task) GetIdx() string {
	return t.idx
}

func (t *Task) GetField(modelField ModelField) (taskField TaskField, err error) {
	for _, field := range t.fields {
		if field.ModelField.Id == modelField.Id {
			return field, err
		}
	}

	return taskField, ErrTaskFieldNotFound
}

//func (n *Neaktor) UpdateTask(taskId int, fields []UpdateTaskField) (err error) {
//	n.apiLimiter.Take()
//
//	updateTasksRequest := UpdateTaskRequest{
//		Fields: fields,
//	}
//
//	updateTasksRequestBytes, err := json.Marshal(updateTasksRequest)
//	if err != nil {
//		return fmt.Errorf("[json.Marshal] error: %w", err)
//	}
//
//	response, err := n.runner.PutJson(HttpRunner.JsonRequestData{
//		Url:   fmt.Sprintf(API_SERVER+"/v1/tasks/%d", taskId),
//		Value: updateTasksRequestBytes,
//		Headers: map[string]string{
//			"Authorization": n.token,
//		},
//	})
//	if err != nil {
//		return fmt.Errorf("/v1/tasks/%d response error: %w", taskId, err)
//	}
//
//	var updateTasksResponse UpdateTasksResponse
//	if err := json.Unmarshal(response.Body(), &updateTasksResponse); err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return fmt.Errorf("unmarshal error: %w", err)
//	}
//	if len(updateTasksResponse.Code) > 0 {
//		return fmt.Errorf("server error: %s", updateTasksResponse.Message)
//	}
//
//	return nil
//}

func (t *Task) UpdateFields(fields []TaskField) error {
	//type UpdateTaskFieldsCurrencyValue struct {
	//	Value    float64 `json:"value,omitempty"`
	//	Currency string  `json:"currency,omitempty"`
	//}

	type UpdateTaskAssignee struct {
		Id   int    `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
	}

	type UpdateTaskField struct {
		Id    string      `json:"id,omitempty"`
		Value interface{} `json:"value,omitempty"`
	}

	type UpdateTaskRequest struct {
		StartDate string              `json:"startDate,omitempty"`
		EndDate   string              `json:"endDate,omitempty"`
		Assignee  *UpdateTaskAssignee `json:"assignee,omitempty"`
		Fields    []UpdateTaskField   `json:"fields,omitempty"`
	}

	type UpdateTasksResponse struct {
		NeaktorErrorResponse
	}

	//

	t.model.neaktor.apiLimiter.Take()

	updateFields := make([]UpdateTaskField, 0)

	for _, field := range fields {
		updateFields = append(updateFields, UpdateTaskField{
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
	response, err := t.model.neaktor.runner.PutJson(HttpRunner.JsonRequestData{
		Url:   fmt.Sprintf(API_SERVER+"/v1/tasks/%d", t.id),
		Value: updateTasksRequestBytes,
		Headers: map[string]string{
			"Authorization": t.model.neaktor.token,
		},
	})
	if err != nil {
		return fmt.Errorf("/v1/tasks/%d response error: %w", t.id, err)
	}

	var updateTasksResponse UpdateTasksResponse
	if err := json.Unmarshal(response.Body(), &updateTasksResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(updateTasksResponse.Code) > 0 {
		return parseErrorCode(updateTasksResponse.Code, updateTasksResponse.Message)
	}

	return err
}

//func (n *Neaktor) UpdateTaskStatus(taskId int, statusId string) (err error) {
//	n.apiLimiter.Take()
//
//	updateTaskStatusRequest := UpdateTaskStatusRequest{
//		Status: statusId,
//	}
//
//	updateTaskStatusRequestBytes, err := json.Marshal(updateTaskStatusRequest)
//	if err != nil {
//		return fmt.Errorf("[json.Marshal] error: %w", err)
//	}
//
//	response, err := n.runner.PostJson(HttpRunner.JsonRequestData{
//		Url:   fmt.Sprintf(API_SERVER+"/v1/tasks/%d/status/change", taskId),
//		Value: updateTaskStatusRequestBytes,
//		Headers: map[string]string{
//			"Authorization": n.token,
//		},
//	})
//	if err != nil {
//		return fmt.Errorf("/v1/tasks/%d/status/change response error: %w", taskId, err)
//	}
//
//	var updateTaskStatusResponse UpdateTaskStatusResponse
//	if err := json.Unmarshal(response.Body(), &updateTaskStatusResponse); err != nil {
//		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
//		return fmt.Errorf("unmarshal error: %w", err)
//	}
//	if len(updateTaskStatusResponse.Code) > 0 {
//		return fmt.Errorf("server error: %s", updateTaskStatusResponse.Message)
//	}
//
//	return nil
//}

func (t *Task) UpdateStatus(status ModelStatus) error {
	type UpdateTaskStatusAssignee struct {
		Id   int    `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
	}

	type UpdateTaskStatusRequest struct {
		Status      string                    `json:"status,omitempty"`
		ConditionId string                    `json:"conditionId,omitempty"`
		Assignee    *UpdateTaskStatusAssignee `json:"assignee,omitempty"`
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
	response, err := t.model.neaktor.runner.PutJson(HttpRunner.JsonRequestData{
		Url:   fmt.Sprintf(API_SERVER+"/v1/tasks/%d/status/change", t.id),
		Value: updateTaskStatusRequestBytes,
		Headers: map[string]string{
			"Authorization": t.model.neaktor.token,
		},
	})
	if err != nil {
		return fmt.Errorf("/v1/tasks/%d/status/change response error: %w", t.id, err)
	}

	var updateTaskStatusResponse UpdateTaskStatusResponse
	if err := json.Unmarshal(response.Body(), &updateTaskStatusResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(updateTaskStatusResponse.Code) > 0 {
		return parseErrorCode(updateTaskStatusResponse.Code, updateTaskStatusResponse.Message)
	}

	return err
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

	response, err := t.model.neaktor.runner.PostJson(HttpRunner.JsonRequestData{
		Url:   fmt.Sprintf(API_SERVER+"/v1/comments/%d", t.id),
		Value: createCommentToTaskRequestBytes,
		Headers: map[string]string{
			"Authorization": t.model.neaktor.token,
		},
	})
	if err != nil {
		return fmt.Errorf("/v1/comments/%d response error: %w", t.id, err)
	}

	var createCommentToTaskResponse CreateCommentToTaskResponse
	if err := json.Unmarshal(response.Body(), &createCommentToTaskResponse); err != nil {
		log.Debugf("response code: %d, response body: %v", response.StatusCode(), string(response.Body()))
		return fmt.Errorf("unmarshaling error: %w", err)
	}
	if len(createCommentToTaskResponse.Code) > 0 {
		return parseErrorCode(createCommentToTaskResponse.Code, createCommentToTaskResponse.Message)
	}

	return err
}
