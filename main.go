package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"net/http"
)

type User struct{
	chat_id int64
	name string
	phone string
	status int
}

var db *sql.DB

func ConnectDB()  {
	var err error
	db, err = sql.Open("mysql", "")
	if err != nil {
		log.Panic(err)
	}
}

func InsertNewUser(user User)  (bool) {
	_, err := db.Exec("insert into users (chat_id, name, status) values (?, ?, ?)", &user)
	if err != nil{
		log.Panic(err)
		return false
	} else {
		return true
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Application started.")
}

var roleMap map[int]string

func main() {

	//TODO: подключение к БД
	ConnectDB()

	// Oбработчик запросов от telegram API
	bot, err := tgbotapi.NewBotAPI("g")

	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true
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
		if update.Message != nil {
			chatID := update.Message.Chat.ID
			userName := update.Message.Chat.LastName + " " + update.Message.Chat.FirstName
			// Определим ответ по умолчанию
			msgText := "Здравствуйте " + update.Message.Chat.FirstName + "! Я Ваш ассистент-бот Gruzov. Для начала работы выберите:"

			rows, err := db.Query("select * from users where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
			defer rows.Close()

			if rows.Next() == false {

				newuser := User{chat_id:chatID, name:userName, status:0}
				result := InsertNewUser(newuser)
				if result == true {
					continue
				}
			}

			msg := tgbotapi.NewMessage(chatID, msgText)
			// команды /start и /help
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "help":
					msgText = "type /start"
				case "start":
					var StartKeyboard = tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData("Заказчик","0"),
							tgbotapi.NewInlineKeyboardButtonData("Перевозчик","1"),
						),
					)

					msg.ReplyMarkup = StartKeyboard
				}
			}
			sm, _ := bot.Send(msg)
			lastID = sm.MessageID
		} else {
			if  lastID != 0 && update.CallbackQuery != nil {
				bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID,update.CallbackQuery.Data))
				bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Укажите телефон"))
				role := update.CallbackQuery.Data
				/*roleMap[update.CallbackQuery.From.ID] = role*/
				step1Msg := "Выберите желаемое действие"
				step1BtnText_1 := "Создать новую заявку"
				step1BtnText_2 := "Все Ваши заявки"
				//step1BtnData_1 := "create_ticket"
				//step1BtnData_2 := "show_tickets"

				if role == "1" {
					step1Msg = "Выберите заявку и предложите цену."
					step1BtnText_1 = "Предложить цену"
					step1BtnText_2 = "Ваши заявки на исполнении"
					//step1BtnData_1 = "get_tickets"
					//step1BtnData_2 = "get_processing"
				}

				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, step1Msg)
				butt := tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton(step1BtnText_1),
					tgbotapi.NewKeyboardButton(step1BtnText_2),
					tgbotapi.NewKeyboardButtonContact("Укажите телефон"),
					)
				keyb := tgbotapi.NewReplyKeyboard(butt)
				msg.ReplyMarkup = &keyb
				sm, _ := bot.Send(msg)
				lastID = sm.MessageID
			}
		}

	}
	// проверка есть ли пользователь в БД по UserID
	// если есть, смотрим роль и вытаскиваем данные в зависимости от нее
	// если новый пользователь:
	// запоминаем UserID и телефон
	// предлагаем выбрать роль
	// если указал роль, дописываем ее в БД
}