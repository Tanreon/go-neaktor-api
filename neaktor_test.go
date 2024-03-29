package neaktor_api

import (
	"log"
	"strings"
	"testing"

	requrl "github.com/wangluozhe/requests/url"
)

func TestNeaktorApi(t *testing.T) {
	t.Run("NeaktorApi", func(t *testing.T) {
		neaktor := NewNeaktor(*requrl.NewRequest(), "t1o2k3e4n5", 100)

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

	t.Run("MustNeaktorApi", func(t *testing.T) {
		neaktor := NewNeaktor(*requrl.NewRequest(), "t1o2k3e4n5", 100)

		model := neaktor.MustGetModelByTitle("Заказ")

		searchModelStatus := model.MustGetStatus("новый заказ")
		newModelStatus := model.MustGetStatus("ошибочный заказ")

		emailModelField := model.MustGetField("email")
		passwordModelField := model.MustGetField("пароль")

		tasks := model.MustGetTasksByStatus(searchModelStatus)

		for _, task := range tasks {
			passwordTaskField := TaskField{
				ModelField: passwordModelField,
				Value:      "qwerty123",
			}

			emailTaskField := task.MustGetField(emailModelField)
			emailTaskField.Value = "admin@gmail.com"

			log.Printf("task id: %q, idx: %q, email field: %q", task.GetId(), task.GetIdx(), emailTaskField)

			if strings.EqualFold(emailTaskField.Value.(string), "admin@google.com") {
				log.Printf("updating fields")
				task.MustUpdateFields([]TaskField{passwordTaskField, emailTaskField})

				//

				log.Printf("updating status")
				task.MustUpdateStatus(newModelStatus)

				//

				log.Printf("adding comment")
				task.MustAddComment("emailTaskField value fixed")
			}
		}
	})
}
