package neaktor_api

import (
	"strings"
	"testing"

	HttpRunner "github.com/Tanreon/go-http-runner"
	NetworkRunner "github.com/Tanreon/go-network-runner"
	log "github.com/sirupsen/logrus"
)

func TestNeaktorApi(t *testing.T) {
	t.Run("NeaktorApi", func(t *testing.T) {
		dialOptions := NetworkRunner.DialOptions{
			DialTimeout:  120,
			RelayTimeout: 60,
		}
		directDialer, err := NetworkRunner.NewDirectDialer(dialOptions)
		if err != nil {
			t.Fatal(err)
		}
		runner, err := HttpRunner.NewDirectHttpRunner(directDialer)
		if err != nil {
			t.Fatal(err)
		}
		neaktor := NewNeaktor(&runner, "t1o2k3e4n5", 100)

		model, err := neaktor.GetModelByTitle("Заказ")
		if err != nil {
			t.Fatal(err)
		}

		searchModelStatus, err := model.GetStatus("новый заказ")
		if err != nil {
			t.Fatal(err)
		}
		newModelStatus, err := model.GetStatus("ошибочный заказ")
		if err != nil {
			t.Fatal(err)
		}

		emailModelField, err := model.GetField("email")
		if err != nil {
			t.Fatal(err)
		}
		passwordModelField, err := model.GetField("пароль")
		if err != nil {
			t.Fatal(err)
		}

		tasks, err := model.GetTasksByStatus(searchModelStatus)
		if err != nil {
			t.Fatal(err)
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
	})
}
