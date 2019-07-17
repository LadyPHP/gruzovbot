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
	address_to    string
	address_from  string
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

func getStep(chatID int64) (step int) {
	users, err := db.Query("select status from users where chat_id=?", chatID)
	if err != nil {
		log.Panic(err)
	}
	defer users.Close()

	if users.Next() != false {
		var user User
		err = users.Scan(&user.status)
		if err != nil {
			log.Panic(err)
		}
		step = user.status
	}

	return step
}

func UpdateTicket(chatID int64, step int, field string, value string) bool {
	if field == "weight" || field == "volume" || field == "length" {
		updValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Panic(err)
		}
		_, err = db.Exec("update tickets set "+field+"=? where customer_id=? and status=1", updValue, chatID)
		if err != nil {
			log.Panic(err)
		}
	} else {
		updValue := value
		_, err := db.Exec("update tickets set "+field+"=? where customer_id=? and status=1", updValue, chatID)
		if err != nil {
			log.Panic(err)
		}
	}

	step += 1
	_, err := db.Exec("update users set status=? where chat_id=?", step, chatID)
	if err != nil {
		log.Panic(err)
	}
	return true
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
	buttonInline := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("test", "test"),
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
		message = "Укажите адрес погрузки"
		// создаем запись в БД о новой заявке
		result, err := db.Exec("insert into tickets (customer_id, status) values (?, 1)", chatID)
		_, err = result.LastInsertId()
		if err != nil {
			log.Panic(err)
		}
		_, err = db.Exec("update users set status=3 where chat_id=?", chatID)
		if err != nil {
			log.Panic(err)
		}
	case 3:
		message = "Адрес выгрузки"
		UpdateTicket(chatID, step, "address_to", data)
	case 4:
		message = "Дата и время погрузки"
		UpdateTicket(chatID, step, "address_from", data)
	case 5:
		message = "Тип автомобиля"
		buttonInline = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("закрытый", `{"step":6, "data":"закрытый"}`),
				tgbotapi.NewInlineKeyboardButtonData("открытый", `{"step":6, "data":"открытый"}`),
				tgbotapi.NewInlineKeyboardButtonData("специальный", `{"step":6, "data":"специальный"}`),
			),
		)
		UpdateTicket(chatID, step, "date", data)
	case 6:
		message = "Тип погрузки"
		UpdateTicket(chatID, step, "car_type", data)
		buttonInline = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("верхняя", `{"step":7, "data":"верхняя"}`),
				tgbotapi.NewInlineKeyboardButtonData("задняя", `{"step":7, "data":"задняя"}`),
				tgbotapi.NewInlineKeyboardButtonData("боковая", `{"step":7, "data":"боковая"}`),
			),
		)
	case 7:
		message = "Вес груза, кг"
		UpdateTicket(chatID, step, "shipment_type", data)
	case 8:
		message = "Объем груза, м3"
		UpdateTicket(chatID, step, "weight", data)
	case 9:
		message = "Максимальная длина, м (если известна)"
		UpdateTicket(chatID, step, "volume", data)
	case 10:
		message = "Дополнительная информация"
		UpdateTicket(chatID, step, "length", data)
	case 11:
		result := UpdateTicket(chatID, step, "comments", data)
		if result == true {
			// обновляем шаг пользователя
			_, err := db.Exec("update users set status=? where chat_id=?", step+1, chatID)
			if err != nil {
				log.Panic(err)
			}

			tickets, err := db.Query("select ticket_id, date, address_to, address_from, comments, car_type, shipment_type, weight, volume, length from tickets where customer_id=? and status = 1", chatID)
			if err != nil {
				log.Panic(err)
			}
			defer tickets.Close()

			if tickets.Next() == false {
				log.Panic(err)
			}
			// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
			var ticket Ticket

			err = tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
			if err != nil {
				log.Panic(err)
			}

			message = "Проверьте информацию и подтвердите публикацию заявки. \n" +
				fmt.Sprintln("№ + ", ticket.ticket_id,
					", \nДата и время: ", ticket.date,
					", \nАдрес погрузки: ", ticket.address_to,
					", \nАдрес выгрузки: ", ticket.address_from,
					", \nКомментарий: ", ticket.comments,
					", \nТип автомобиля: ", ticket.car_type,
					", \nТип погрузчика: ", ticket.shipment_type,
					", \nВес (кг): ", ticket.weight,
					", \nОбъем (м3): ", ticket.volume,
					", \nМакс.длина (м): ", ticket.length)
			buttonInline = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Опубликовать", fmt.Sprintf(`{"step":12, "data":%d}`, ticket.ticket_id)),
					//tgbotapi.NewInlineKeyboardButtonData("Редактировать", fmt.Sprintf(`{"step":13, "data":%d}`, ticket.ticket_id)),
					tgbotapi.NewInlineKeyboardButtonData("Отменить", fmt.Sprintf(`{"step":14, "data":%d}`, ticket.ticket_id)),
				),
			)
		}
	case 12:
		_, err := db.Exec("update tickets set status=2 where customer_id=? and ticket_id=? and status=1", chatID, data)
		if err != nil {
			log.Panic(err)
		}
		message = "Заявка опубликована. Ожидайте отклики от исполнителей."
		/*extMsg := tgbotapi.NewMessage(-1001370763028, "Новая заявка")
		bidButton := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("1.com", "http://t.me/devnil_bot?help"),
				//tgbotapi.NewInlineKeyboardButtonSwitch("2sw","open devnil_bot"),
				tgbotapi.NewInlineKeyboardButtonData("Предложить цену", fmt.Sprintf("bid %d", ticketID)),
			),
		)*/
	case 13:
		//
	case 14:
		_, err := db.Exec("update tickets set status=0 where customer_id=? and ticket_id=?", chatID, data)
		if err != nil {
			log.Panic(err)
		}
		message = "Заявка отменена."
	}

	msg = tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = button
	msg.ReplyMarkup = buttonInline
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
			} else {
				switch update.Message.Text {
				case "Помощь":
					msg = Commands(chatID, userName, "help")
				case "Создать заявку":
					msg = ticketHandlerClient(2, "", chatID)
				case "История заявок":
					msg = Commands(chatID, userName, "history")
				case "Изменить роль":
					msg = Commands(chatID, userName, "role")
				default:
					step := getStep(chatID)
					msg = ticketHandlerClient(step, update.Message.Text, chatID)
				}
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
