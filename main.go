package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"net/http"
	"os"
)

type Config struct {
	TelegramBotToken  string
	TelegramDebugMode bool
	DBConnectUrl      string
}

var db *sql.DB
var configuration Config

type User struct {
	chat_id int64
	name    string
	phone   string
	status  int
}

func GetConfig() {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&configuration)
	if err != nil {
		log.Panic(err)
	}
}

func ConnectDB() {
	var err error
	db, err = sql.Open("mysql", configuration.DBConnectUrl)
	if err != nil {
		log.Panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Application started.")
}

var roleMap map[int]string

func main() {
	GetConfig() // подключение конфига
	ConnectDB() // подключение к БД

	// Oбработчик запросов от telegram API
	bot, err := tgbotapi.NewBotAPI(configuration.TelegramBotToken)

	if err != nil {
		log.Panic(err)
	}
	bot.Debug = configuration.TelegramDebugMode
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	//updates, err := bot.GetUpdatesChan(u)
	updates := bot.ListenForWebhook("/" + bot.Token)

	if err != nil {
		log.Panic(err)
	}

	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8087", nil)

	// В канал updates будут приходить все новые сообщения
	lastID := 0
	for update := range updates {
		if update.Message != nil { // если поступило в ответ сообщение
			chatID := update.Message.Chat.ID
			if update.Message.IsCommand() { // если это команда
				switch update.Message.Command() {
				case "start":
					// Определяем входные параметры для чата
					userName := update.Message.Chat.LastName + " " + update.Message.Chat.FirstName
					msgText := "Здравствуйте " + update.Message.Chat.FirstName + "! Я Ваш ассистент-бот."
					msg := tgbotapi.NewMessage(chatID, msgText)
					sm, _ := bot.Send(msg)
					lastID = sm.MessageID
					// Определяем есть ли пользователь в базе
					rows, err := db.Query("select * from users where chat_id=?", chatID)
					if err != nil {
						log.Panic(err)
					}
					defer rows.Close()

					// если нет, то добавляем
					if rows.Next() == false {
						_, err := db.Exec("insert into users (chat_id, name, status) values (?, ?, ?)", chatID, userName, 0)
						if err != nil {
							log.Panic(err)
						}

						msgText = "Для начала работы отправьте ваш телефон (кнопка внизу)."
						msg = tgbotapi.NewMessage(chatID, msgText)
						msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
							tgbotapi.NewKeyboardButtonRow(
								tgbotapi.NewKeyboardButtonContact("Отправить телефон"),
							),
						)
					} else {
						msgText = "Выберите:"
						msg = tgbotapi.NewMessage(chatID, msgText)
						msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
							tgbotapi.NewInlineKeyboardRow(
								tgbotapi.NewInlineKeyboardButtonData("Заказчик", "0"),
								tgbotapi.NewInlineKeyboardButtonData("Перевозчик", "1"),
							),
						)
					}
					sm, _ = bot.Send(msg)
					lastID = sm.MessageID

				case "step1":

				}

			}
			Phone := update.Message.Contact.PhoneNumber
			msgText := "TEST"
			if len(Phone) > 0 {
				_, err := db.Exec("update users set phone=? where chat_id=?", Phone, chatID)
				if err != nil {
					log.Panic(err)
				}

				msgText = update.Message.Contact.PhoneNumber
			}
			msg := tgbotapi.NewMessage(chatID, msgText)
			sm, _ := bot.Send(msg)
			lastID = sm.MessageID
		} else {
			if lastID != 0 && update.CallbackQuery != nil {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Вы что-то отправили. Я что-то отвечаю.")
				sm, _ := bot.Send(msg)
				lastID = sm.MessageID
			}
		}
	}
}
