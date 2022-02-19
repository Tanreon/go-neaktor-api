package neaktor_api

import (
	"strings"
	"testing"

	httpRunner "github.com/Tanreon/go-http-runner"
	networkRunner "github.com/Tanreon/go-network-runner"
	log "github.com/sirupsen/logrus"
)

func TestNeaktorApi(t *testing.T) {
	dialer, err := networkRunner.NewDirectDialer()
	if err != nil {
		panic(err)
	}
	runner, err := httpRunner.NewDirectHttpRunner(dialer)
	if err != nil {
		panic(err)
	}
	neaktor := NewNeaktor(&runner, "test-token", 100)

	model, err := neaktor.GetModelByTitle("Заказ")
	if err != nil {
		panic(err)
	}

	searchModelStatus, err := model.GetStatus("новый заказ")
	if err != nil {
		panic(err)
	}
	newModelStatus, err := model.GetStatus("ошибочный заказ")
	if err != nil {
		panic(err)
	}

	emailModelField, err := model.GetField("email")
	if err != nil {
		panic(err)
	}
	passwordModelField, err := model.GetField("пароль")
	if err != nil {
		panic(err)
	}

	tasks, err := model.GetTasksByStatus(searchModelStatus)
	if err != nil {
		panic(err)
	}

	for _, task := range tasks {
		passwordTaskField := TaskField{
			ModelField: passwordModelField,
			Value:      "qwerty123",
		}

		emailTaskField, err := task.GetField(emailModelField)
		if err != nil {
			panic(err)
		}
		emailTaskField.Value = "admin@gmail.com"

		log.Printf("task id: %q, idx: %q, email field: %q", task.GetId(), task.GetIdx(), emailTaskField)

		if strings.EqualFold(emailTaskField.Value.(string), "admin@google.com") {
			log.Printf("updating fields")
			task.UpdateFields([]TaskField{passwordTaskField, emailTaskField})

			//

			log.Printf("updating status")
			task.UpdateStatus(newModelStatus)

			//

			log.Printf("adding comment")
			task.AddComment("emailTaskField value fixed")
		}
	}

	t.Run("main", func(t *testing.T) {
		//
	})
}
