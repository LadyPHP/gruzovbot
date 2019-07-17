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
	"strconv"
)

var db *sql.DB
var configuration Config
var msg tgbotapi.MessageConfig

type Config struct {
	TelegramBotToken  string
	TelegramDebugMode bool
	TelegramAPIMode   string
	DBConnectUrl      string
}

type User struct {
	chat_id int64
	name    string
	phone   string
	status  int
	role    int
}

type stepData struct {
	Step int
	Data string
	Role int
}

type Ticket struct {
	ticket_id     uint
	date          string
	address       string
	options       string
	comments      string
	status        int
	customer_id   int
	car_type      string
	shipment_type string
	weight        float64
	volume        float64
	length        float64
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

func CheckUser(chatID int64) bool {
	users, err := db.Query("select * from users where chat_id=?", chatID)
	if err != nil {
		log.Panic(err)
	}
	defer users.Close()

	return users.Next()
}

func Commands(chatID int64, user string, command string) (msg tgbotapi.MessageConfig) {
	message := "Здравствуйте " + user + "! \n"
	button := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Помощь"),
		),
	)

	switch command {
	case "start":
		message = message + "Я Ваш ассистент-бот. С моей помощью Вы можете работать с заказами на перевозку. \n\n Для продолжения работы запустите команду /registration"
	case "help":
		message = "Что умеет этот бот: \n " +
			"Регистрировать новых пользователей /registration \n" +
			"Выбирать роль - /role \n" +
			"Для заказчика: \n" +
			"Создавать заявки на перевозку - /create \n" +
			"Отслеживать статус заявок /history \n" +
			"Для исполнителя: \n" +
			"Получать уведомления о новых заказах - /notification \n" +
			"Предлагать тип ставки и цену сделки - /deal"
	case "registration":
		result := CheckUser(chatID)
		if result == false {
			_, err := db.Exec("insert into users (chat_id, name, status) values (?, ?, ?)", chatID, user, 0)
			if err != nil {
				log.Panic(err)
			}

			message = "Отлично, а теперь для завершения регистрации отправьте ваш телефон (кнопка внизу)."
			button = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButtonContact("Отправить телефон"),
				),
			)
		} else {
			message = "Вы уже зарегистрированы. Для продолжения работы воспользуйтесь подсказкой /help"
		}
	case "create":
		message = "Адрес погрузки"
	}

	msg = tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = button
	return msg
}

func ticketHandlerClient(step int, data string, chatID int64) (msg tgbotapi.MessageConfig) {
	message := ""
	button := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Помощь"),
		),
	)

	switch step {
	case 1:
		_, err := db.Exec("update users set status=?, role=0 where chat_id=?", step, chatID)
		if err != nil {
			log.Panic(err)
		}
		message = "Отлично, при необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню.\n\n" +
			"Для продолжения работы с заявками выберите в меню одно из действий."
		button = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Создать заявку"),
				tgbotapi.NewKeyboardButton("История заявок"),
				tgbotapi.NewKeyboardButton("Изменить роль"),
			),
		)
	case 2:

	}

	msg = tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = button
	return msg
}

func ticketHandlerExecutant(step int, data string, chatID int64) (msg tgbotapi.MessageConfig) {
	message := ""
	switch step {
	case 1:
		_, err := db.Exec("update users set status=?, role=1 where chat_id=?", step, chatID)
		if err != nil {
			log.Panic(err)
		}
		message = "Отлично, при необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню.\n\n"
	case 2:

	}

	msg = tgbotapi.NewMessage(chatID, message)
	//msg.ReplyMarkup = button
	return msg
}

func main() {
	GetConfig() // подключение конфига
	ConnectDB() // подключение к БД

	// Oбработчик запросов от telegram API
	bot, err := tgbotapi.NewBotAPI(configuration.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = configuration.TelegramDebugMode
	if bot.Debug == true {
		log.Printf("Authorized on account %s", bot.Self.UserName)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// тип взаимодействия с API (переключается в config.json: "TelegramAPIMode" : "channel" или "webhook")
	updates := bot.ListenForWebhook("/" + bot.Token)
	if configuration.TelegramAPIMode == "channel" {
		updates, err = bot.GetUpdatesChan(u)
	}

	if err != nil {
		log.Panic(err)
	}

	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8087", nil)

	// В канал updates будут приходить все новые сообщения
	for update := range updates {
		// Блок обработки поступающих в ответ сообщений

		if update.Message != nil {
			chatID := update.Message.Chat.ID
			userName := update.Message.Chat.LastName + " " + update.Message.Chat.FirstName

			// Обработка команд
			if update.Message.IsCommand() {
				msg = Commands(chatID, userName, update.Message.Command())
			} else if update.Message.Text == "Помощь" {
				msg = Commands(chatID, userName, "help")
			} else if update.Message.Text == "Создать заявку" {
				msg = Commands(chatID, userName, "create")
			} else if update.Message.Text == "История заявок" {
				msg = Commands(chatID, userName, "history")
			} else if update.Message.Text == "Изменить роль" {
				msg = Commands(chatID, userName, "role")
			}

			// Обработка, если пришло сообщение-контакт
			if update.Message.Contact != nil {
				// вытягиваем номер телефона из сообщения
				Phone, err := strconv.ParseUint(update.Message.Contact.PhoneNumber, 0, 64)
				if err != nil {
					log.Panic(err)
				}
				// записываем номер телефона в базу и активируем пользователя со статусом = 1
				_, err = db.Exec("update users set phone=?, status=1 where chat_id=?", Phone, chatID)
				if err != nil {
					log.Panic(err)
				}

				// отправляем сообщение, чтобы выбрал роль
				msgText := "Выберите:"
				msg = tgbotapi.NewMessage(chatID, msgText)
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Заказчик", `{"step":1, "role":0}`),
						tgbotapi.NewInlineKeyboardButtonData("Перевозчик", `{"step":1, "role":1}`),
					),
				)
			}

			// Обработка остальных текстовых сообщений
		} else if update.CallbackQuery != nil {
			chatID := update.CallbackQuery.Message.Chat.ID

			dataMap := stepData{}
			json.Unmarshal([]byte(update.CallbackQuery.Data), &dataMap)

			step := dataMap.Step
			data := dataMap.Data
			role := dataMap.Role

			if role == 0 {
				msg = ticketHandlerClient(step, data, chatID)
			} else {
				msg = ticketHandlerExecutant(step, data, chatID)
			}
		}

		bot.Send(msg)
	}
}
