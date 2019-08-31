package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/telegram-bot-api.v4"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var db *sql.DB
var configuration Config
var msg tgbotapi.MessageConfig

//var bot = botImplement()

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
	ticket_id         uint
	date              string
	address_to        string
	address_from      string
	options           string
	comments          string
	status            int
	customer_id       int
	car_type          string
	shipment_type     string
	weight            float64
	volume            float64
	length            float64
	chanel_message_id int
}

type Bid struct {
	BidID       uint
	PerformerID int
	TicketID    int
	Status      int
	Price       string
	TypePrice   int
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

func botImplement() (bot *tgbotapi.BotAPI) {
	bot, err := tgbotapi.NewBotAPI(configuration.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	return bot
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Application started.")
}

func getStep(chatID int64) (step int) {
	step = -1
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

	return
}

func changeRole(chatID int64) (msg tgbotapi.MessageConfig) {
	message := "Выберите:"
	msg = tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Заказчик", `{"step":1, "role":0}`),
			tgbotapi.NewInlineKeyboardButtonData("Перевозчик", `{"step":100, "role":1}`),
		),
	)

	return
}

func UpdateTicket(chatID int64, step int, field string, value string) (bool, string) {
	if field == "weight" || field == "volume" || field == "length" {
		updValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false, "Не удалось записать ответ. Введите число. Другие символы не допустимы. Если точное значение не известно, то укажите приблизительное. А далее можно будет оставить комментарий для исполнителя (я далее отдельно это предложу сделать)."
		}
		_, err = db.Exec("update tickets set "+field+"=? where customer_id=? and status=1", updValue, chatID)
		if err != nil {
			log.Panic(err)
		}
	} else if field == "date" {
		layout := "2006-01-02 15:04"
		updValue, err := time.Parse(layout, value)
		if err != nil {
			return false, "Не удалось записать дату и время. Введите по формату ГГГГ-мм-дд чч:мм, например, 2019-06-12 10:00. Другие символы не допустимы. Если точное значение не известно, то укажите приблизительное. А далее можно будет оставить комментарий для исполнителя (я далее отдельно это предложу сделать)."
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
	return true, ""
}

func getTicketInfo(chatID int64, ticketID string, status int) (tickets *sql.Rows) {
	tickets, err := db.Query("select ticket_id, date, address_to, address_from, comments, car_type, shipment_type, weight, volume, length from tickets where status = ?", status)

	if status == 1 {
		tickets, err = db.Query("select ticket_id, date, address_to, address_from, comments, car_type, shipment_type, weight, volume, length from tickets where status = ? and customer_id=? order by ticket_id desc limit 1", status, chatID)
	}

	if status == 0 {
		tickets, err = db.Query("select ticket_id, status, date, address_to, address_from, comments, car_type, shipment_type, weight, volume, length from tickets where customer_id=? and date != '' order by ticket_id asc", chatID)
	}

	if ticketID != "0" {
		tickets, err = db.Query("select ticket_id, date, address_to, address_from, comments, car_type, shipment_type, weight, volume, length, chanel_message_id from tickets where ticket_id=?", ticketID)
	}

	//defer tickets.Close()

	if tickets.Next() == false {
		log.Println(err)
	}

	return tickets
}

func Commands(chatID int64, user string, command string, data string) (msg tgbotapi.MessageConfig) {
	message := "Здравствуйте " + user + "! \n"
	var button tgbotapi.ReplyKeyboardMarkup
	var buttonInline tgbotapi.InlineKeyboardMarkup
	var ticketID int64

	if len(data) > 7 {
		data = strings.ReplaceAll(data, "/start ", "")
		ticketID, _ = strconv.ParseInt(data, 0, 64)
		if ticketID > 0 {
			command = "registration"
		}
	}

	switch command {
	case "start":
		message = message + "Я Ваш ассистент-бот. С моей помощью Вы можете работать с заказами на перевозку. \n\n Для продолжения работы запустите команду /registration"
	case "help":
		message = "Что умеет этот бот: \n " +
			"Регистрировать новых пользователей /registration \n" +
			"Выбирать роль - по кнопке в меню 'Изменить роль' \n" +
			"Для заказчика: \n" +
			"Создавать заявки на перевозку - по кнопке в меню 'Создать новую заявку' \n" +
			"Отслеживать статус заявок - по кнопке в меню 'История заявок' \n" +
			"Для исполнителя: \n" +
			"Получать уведомления о новых заказах - для этого подпишитесь на канал @gruzov_v \n" +
			"Предлагать тип ставки и цену сделки - для этого просто откликнитесь на заявку и выберите тип рассчета: фиксированный или почасовую ставку."
	case "registration":
		step := getStep(chatID)
		if step <= 0 {
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
			message = "Вы уже зарегистрированы. Выберите роль:"
			buttonInline = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Заказчик", `{"step":1, "role":0}`),
					tgbotapi.NewInlineKeyboardButtonData("Перевозчик", `{"step":100, "role":1}`),
				),
			)
		}

		// если при переходе в бота был указан id заявки, то считаем это откликом от перевозчика
		// и выдаем ему сразу же соответствующий функционал
		if ticketID > 0 {
			_, err := db.Exec("update users set role=1 where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}

			message = fmt.Sprintln("Вы уже зарегистрированы как перевозчик. Для продолжения работы с заявкой № ", ticketID, "нажмите кнопку.")
			buttonInline = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Предложить цену", fmt.Sprintf(`{"step":102, "data":"%d"}`, ticketID)),
				),
			)
		}
	case "crush_application":
		log.Panic()
	}

	msg = tgbotapi.NewMessage(chatID, message)

	if button.Keyboard != nil {
		msg.ReplyMarkup = button
	}
	if buttonInline.InlineKeyboard != nil {
		msg.ReplyMarkup = buttonInline
	}

	return msg
}

func ticketHandlerClient(step int, data string, chatID int64) (msg tgbotapi.MessageConfig) {
	var message string
	var button tgbotapi.ReplyKeyboardMarkup
	var buttonInline tgbotapi.InlineKeyboardMarkup

	bot := botImplement()

	switch step {
	case 1:
		_, err := db.Exec("update users set status=?, role=0 where chat_id=?", step, chatID)
		if err != nil {
			log.Panic(err)
		}
		message = "Отлично, теперь Вы - заказчик! При необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню.\n\n" +
			"Для продолжения работы с заявками выберите в меню одно из действий."
		button = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Создать заявку"),
				tgbotapi.NewKeyboardButton("Изменить роль"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("История заявок"),
				tgbotapi.NewKeyboardButton("Скачать отчет (заказчик)"),
			),
		)
	case 2:
		message = "Укажите адрес погрузки"
		if len(data) < 1 {
			tickets, err := db.Query("select ticket_id from tickets where customer_id=? and status=1 limit 1", chatID)
			if err != nil {
				log.Panic(err)
			}
			defer tickets.Close()
			if tickets.Next() == false {
				// создаем запись в БД о новой заявке
				result, err := db.Exec("insert into tickets (customer_id, status) values (?, 1)", chatID)
				_, err = result.LastInsertId()
				if err != nil {
					log.Panic(err)
				}
			} else {
				//TODO: дописать предложить продолжить заполнение заявки или удалить ее
				var ticket Ticket
				err := tickets.Scan(&ticket.ticket_id)
				if err != nil {
					log.Panic(err)
				}
				message = "У вас уже есть заявка, которую вы начали заполнять. Хотите заполнить ее заново?"
				buttonInline = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Да", fmt.Sprintf(`{"step":2, "data":"%d"}`, ticket.ticket_id)),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Нет, отменить прежнюю", fmt.Sprintf(`{"step":14, "data":"%d"}`, ticket.ticket_id)),
					),
				)
			}
		} else {
			UpdateTicket(chatID, step, "status", "1")
		}

		_, err := db.Exec("update users set status=3 where chat_id=?", chatID)
		if err != nil {
			log.Panic(err)
		}
	case 3:
		message = "Адрес выгрузки"
		UpdateTicket(chatID, step, "address_to", data)
	case 4:
		message = "Дата и время погрузки (например, 2019-06-12 10:00)"
		UpdateTicket(chatID, step, "address_from", data)
	case 5:
		_, message = UpdateTicket(chatID, step, "date", data)
		if len(message) < 1 {
			message = "Тип автомобиля (открытый, закрытый, рефрижератор и т.п.)"
		}
	case 6:
		message = "Тип погрузки (верхняя, задняя, боковая и т.п.)"
		UpdateTicket(chatID, step, "car_type", data)
	case 7:
		message = "Вес груза, кг"
		UpdateTicket(chatID, step, "shipment_type", data)
	case 8:
		_, message = UpdateTicket(chatID, step, "weight", data)
		if len(message) < 1 {
			message = "Объем груза, м3"
		}
	case 9:
		_, message = UpdateTicket(chatID, step, "volume", data)
		if len(message) < 1 {
			message = "Максимальная длина, м (если известна)"
		}
	case 10:
		_, message = UpdateTicket(chatID, step, "length", data)
		if len(message) < 1 {
			message = "Дополнительная информация"
		}
	case 11:
		result, _ := UpdateTicket(chatID, step, "comments", data)
		if result == true {
			// обновляем шаг пользователя
			_, err := db.Exec("update users set status=? where chat_id=?", step+1, chatID)
			if err != nil {
				log.Panic(err)
			}

			tickets := getTicketInfo(chatID, "0", 1)
			defer tickets.Close()

			ticket := new(Ticket)
			err = tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
			if err != nil {
				log.Panic(err)
			}
			message = "Проверьте информацию и подтвердите публикацию заявки. \n" +
				fmt.Sprintln("№", ticket.ticket_id,
					", \nДата и время: ", ticket.date,
					", \nАдрес погрузки: ", ticket.address_to,
					", \nАдрес выгрузки: ", ticket.address_from,
					", \nТип автомобиля: ", ticket.car_type,
					", \nТип погрузчика: ", ticket.shipment_type,
					", \nВес (кг): ", ticket.weight,
					", \nОбъем (м3): ", ticket.volume,
					", \nМакс.длина (м): ", ticket.length,
					", \nКомментарий: ", ticket.comments)
			buttonInline = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Опубликовать", fmt.Sprintf(`{"step":12, "data":"%d"}`, ticket.ticket_id)),
					//tgbotapi.NewInlineKeyboardButtonData("Редактировать", fmt.Sprintf(`{"step":13, "data":"%d"}`, ticket.ticket_id)),
					tgbotapi.NewInlineKeyboardButtonData("Отменить", fmt.Sprintf(`{"step":14, "data":"%d"}`, ticket.ticket_id)),
				),
			)
		}
	case 12:
		_, err := db.Exec("update tickets set status=2 where customer_id=? and ticket_id=? and status<2", chatID, data)
		if err != nil {
			log.Panic(err)
		}
		message = "Заявка опубликована. Ожидайте отклики от исполнителей."
	case 13:
		//TODO: возможность редактирования заявки, если status < 2
	case 14:
		_, err := db.Exec("update tickets set status=0 where customer_id=? and ticket_id=? and status<=2", chatID, data)
		if err != nil {
			log.Panic(err)
		}
		message = "Заявка отменена."

		tickets := getTicketInfo(chatID, data, 0)
		defer tickets.Close()

		ticket := new(Ticket)
		err = tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.chanel_message_id)
		if err != nil {
			log.Panic(err)
		}

		extMsg := tgbotapi.NewEditMessageReplyMarkup(-1001370763028, ticket.chanel_message_id, tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Заявка отменена", "cancel"))))
		bot.Send(extMsg)
	case 15:
		//TODO: копировать заявку - допоилить редактирование некоторых параметров (а не всех по цепочке)
		//TODO: проверить, что скопированная заявка меняет только свои данные (а не все, у которых status = 1)
		result, err := db.Exec("insert into tickets (address_to, address_from, date, options, comments, status, customer_id, car_type, shipment_type, weight, volume, length) select address_to, address_from, '', options, comments, 1, customer_id, car_type, shipment_type, weight, volume, length from tickets where customer_id =? and ticket_id =?", chatID, data)
		InsertTicket, err := result.LastInsertId()
		if err != nil {
			log.Panic(err)
		}
		message = "Заявка скопирована. Желаете отредактировать заявку?"
		buttonInline = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Да", fmt.Sprintf(`{"step":2, "data":"%d"}`, InsertTicket)),
				tgbotapi.NewInlineKeyboardButtonData("Нет, сразу опубликовать", fmt.Sprintf(`{"step":12, "data":"%d"}`, InsertTicket)),
			),
		)

	case 16: // Итория заявок
		tickets := getTicketInfo(chatID, "0", 0)
		defer tickets.Close()

		ticketsRows := make([]*Ticket, 0)

		for tickets.Next() {
			// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
			ticket := new(Ticket)
			err := tickets.Scan(&ticket.ticket_id, &ticket.status, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
			if err != nil {
				log.Panic(err)
			}

			ticketsRows = append(ticketsRows, ticket)
		}

		if len(ticketsRows) > 0 {
			for _, ticket := range ticketsRows {
				btnText1 := "Действия не доступны"
				btnData1 := `{"step":2, "data":"0"}`

				if ticket.status <= 2 {
					btnText1 = "Отменить"
					btnData1 = fmt.Sprintf(`{"step":14, "data":"%d"}`, ticket.ticket_id)
				}
				/*btnText2 := "Копировать"
				btnData2 := fmt.Sprintf(`{"step":15, "data":"%d"}`, ticket.ticket_id)*/
				if ticket.status <= 1 {
					btnText1 = "Опубликовать"
					btnData1 = fmt.Sprintf(`{"step":12, "data":"%d"}`, ticket.ticket_id)
					//btnText2 = "Изменить"
					//btnData2 = fmt.Sprintf(`{"step":13, "data":"%d"}`, ticket.ticket_id)
				}
				/*if ticket.status == 3 {
					btnText1 = "Выполнена?"
					btnData1 = fmt.Sprintf(`{"step":201, "data":"%d"}`, ticket.ticket_id)
				}*/
				messageCustom := fmt.Sprintln(
					"№", ticket.ticket_id,
					", \n Дата и время: ", ticket.date,
					", \n Адрес погрузки: ", ticket.address_to,
					", \n Адрес выгрузки: ", ticket.address_from,
					", \n Тип автомобиля: ", ticket.car_type,
					", \n Тип погрузчика: ", ticket.shipment_type,
					", \n Вес (кг): ", ticket.weight,
					", \n Объем (м3): ", ticket.volume,
					", \n Макс.длина (м): ", ticket.length,
					", \n Комментарий: ", ticket.comments,
				)

				msgCustom := tgbotapi.NewMessage(chatID, messageCustom)
				msgCustom.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(btnText1, btnData1),
						//tgbotapi.NewInlineKeyboardButtonData(btnText2, btnData2),
					),
				)
				bot.Send(msgCustom)
			}
			// обновляем шаг для пользователя
			_, err := db.Exec("update users set status=12 where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
		} else {
			message = "Вы пока не создали ниодной заявки. \n Чтобы начать, нажмите кнопку \"Создать новую\" в меню."
		}
	}

	msg = tgbotapi.NewMessage(chatID, message)

	if button.Keyboard != nil {
		msg.ReplyMarkup = button
	}
	if buttonInline.InlineKeyboard != nil {
		msg.ReplyMarkup = buttonInline
	}

	return msg
}

func ticketHandlerExecutant(step int, data string, chatID int64) (msg tgbotapi.MessageConfig) {
	var message string
	var button tgbotapi.ReplyKeyboardMarkup
	var buttonInline tgbotapi.InlineKeyboardMarkup

	bot := botImplement()

	switch step {
	case 100:
		_, err := db.Exec("update users set status=?, role=1 where chat_id=?", step, chatID)
		if err != nil {
			log.Panic(err)
		}
		message = "Отлично, теперь Вы - перевозчик! При необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню.\n\n" +
			"Для продолжения работы с заявками выберите в меню одно из действий."
		button = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Все новые заявки"),
				tgbotapi.NewKeyboardButton("Изменить роль"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Исполняемые мной"),
				tgbotapi.NewKeyboardButton("Скачать отчет (перевозчик)"),
			),
		)
	case 101:
		tickets := getTicketInfo(chatID, data, 2)
		defer tickets.Close()

		ticketsRows := make([]*Ticket, 0)

		for tickets.Next() {
			// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
			ticket := new(Ticket)
			err := tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
			if err != nil {
				log.Panic(err)
			}
			ticketsRows = append(ticketsRows, ticket)
		}

		if len(ticketsRows) > 0 {
			for _, ticket := range ticketsRows {
				messageCustom := fmt.Sprintln(
					"№", ticket.ticket_id,
					", \n Дата и время: ", ticket.date,
					", \n Адрес погрузки: ", ticket.address_to,
					", \n Адрес выгрузки: ", ticket.address_from,
					", \n Тип автомобиля: ", ticket.car_type,
					", \n Тип погрузчика: ", ticket.shipment_type,
					", \n Вес (кг): ", ticket.weight,
					", \n Объем (м3): ", ticket.volume,
					", \n Макс.длина (м): ", ticket.length,
					", \n Комментарий: ", ticket.comments,
				)

				msgCustom := tgbotapi.NewMessage(chatID, messageCustom)
				msgCustom.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Предложить цену", fmt.Sprintf(`{"step":102, "data":"%d"}`, ticket.ticket_id)),
					),
				)
				bot.Send(msgCustom)
			}
			// обновляем шаг для пользователя
			_, err := db.Exec("update users set status=102 where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
		} else {
			message = "Пока нет новых опубликованных заявок. Попробуйте проверить позже. Либо включите уведомления на канале @gruzov_v"
		}

	case 102:
		// проверяем есть ли уже запрос на исполнение этой заявки этим перевозчиком
		bids, err := db.Query("select bid_id, status, price, type_price from bid where performer_id=? and ticket_id=?", chatID, data)
		if err != nil {
			log.Panic(err)
		}
		defer bids.Close()

		message = "Выберите тип рассчета:"
		buttonInlineTmp := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Почасовой тариф (руб./час)", `{"step":103, "data":"0"}`),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Фиксированная стоимость", `{"step":103, "data":"1"}`),
			),
		)

		if bids.Next() == false {
			// если нет создаем запись в БД о том, что перевозчик взял заявку
			result, err := db.Exec("insert into bid (performer_id, ticket_id, status) values (?, ?, 0)", chatID, data)
			_, err = result.LastInsertId()
			if err != nil {
				log.Panic(err)
			}

			buttonInline = buttonInlineTmp

		} else {
			// иначе уведомляем и спрашиваем хочет ли обновить
			var bid Bid
			err = bids.Scan(&bid.BidID, &bid.Status, &bid.Price, &bid.TypePrice)
			if err != nil {
				log.Panic(err)
			}

			// если уже проставлены значения (status = 2)
			//TODO: пофиксить возможность повторной отправки отклика в зависимости от статуса
			if bid.Status == 2 {
				typePrice := "почасовой тариф (руб./час)"

				if bid.TypePrice == 1 {
					typePrice = "фиксированная стоимость"
				}

				message = "Вы уже откликались на данную заявку " + data + ", указав следующие данные: \n" +
					"\n Тип расчета - " + typePrice +
					"\n Стоимость - " + bid.Price +
					"\n Если хотите обновить данные, то для продолжения выберите тип рассчета: "
				buttonInline = buttonInlineTmp
			} else if bid.Status > 2 && bid.Status < 7 {
				message = "Ваш отклик уже был отправлен заказчику ранее. На этом этапе информацию изменить нельзя. Дождитесь, когда заказчик свяжется с Вами."
			} else if bid.Status < 2 {
				_, err := db.Exec("update bid set status=0 where performer_id=? and bid_id=?", chatID, bid.BidID)
				if err != nil {
					log.Panic(err)
				}

				buttonInline = buttonInlineTmp
			} else if bid.Status == 7 {
				message = "Вы ранее отменили запрос на эту заявку. Если хотите его возобновить, то для продолжения выберите тип рассчета: "
				buttonInline = buttonInlineTmp
				_, err := db.Exec("update bid set status=0 where performer_id=? and status=7", chatID)
				if err != nil {
					log.Panic(err)
				}
			}
		}

		_, err = db.Exec("update users set status=? where chat_id=?", step+1, chatID)
		if err != nil {
			log.Panic(err)
		}
	case 103:
		_, err := db.Exec("update bid set status=1, type_price=? where performer_id=? and status=0", data, chatID)
		if err != nil {
			log.Panic(err)
		}

		_, err = db.Exec("update users set status=? where chat_id=?", step+1, chatID)
		if err != nil {
			log.Panic(err)
		}

		message = "Вы выбрали способ расчета - ставка. Укажите ее размер (руб./час)"

		if data == "1" {
			message = "Вы выбрали фиксированная стоимость способ расчета. Укажите стоимость (в рублях)"
		}
	case 104:
		_, err := db.Exec("update bid set price=?, status=2 where performer_id=? and status=1", data, chatID)
		if err != nil {
			log.Panic(err)
		}

		_, err = db.Exec("update users set status=? where chat_id=?", step+1, chatID)
		if err != nil {
			log.Panic(err)
		}

		message = "Вы указали - " + data + "руб. Отправить запрос заказчику?"
		buttonInline = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Да", `{"step":105, "data":"3"}`),
				tgbotapi.NewInlineKeyboardButtonData("Отменить", `{"step":105, "data":"7"}`),
			),
		)
	case 105:
		_, err := db.Exec("update bid set status=? where performer_id=? and status=2", data, chatID)
		if err != nil {
			log.Panic(err)
		}

		message = "Запрос отменен."

		if data == "3" {
			bids, err := db.Query("select ticket_id, type_price, price, bid_id from bid where status=? and performer_id=?", data, chatID)
			if err != nil {
				log.Panic(err)
			}
			defer bids.Close()

			if bids.Next() != false {
				var bid Bid
				err = bids.Scan(&bid.TicketID, &bid.TypePrice, &bid.Price, &bid.BidID)
				if err != nil {
					log.Panic(err)
				}

				tickets, err := db.Query("select customer_id, ticket_id, date, address_to, address_from, car_type, shipment_type, weight, volume, length, comments from tickets where ticket_id=?", bid.TicketID)
				if err != nil {
					log.Panic(err)
				}
				defer tickets.Close()

				if tickets.Next() != false {
					var ticket Ticket
					err = tickets.Scan(&ticket.customer_id, &ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.comments)
					if err != nil {
						log.Panic(err)
					}

					clientChatID := int64(ticket.customer_id)
					typePrice := "почасовой тариф (руб./час)"

					if bid.TypePrice == 1 {
						typePrice = "фиксированная стоимость"
					}
					messageClient := "Новый отклик на Вашу заявку " +
						fmt.Sprintln(
							"№", ticket.ticket_id,
							", \n Дата и время: ", ticket.date,
							", \n Адрес погрузки: ", ticket.address_to,
							", \n Адрес выгрузки: ", ticket.address_from,
							", \n Тип автомобиля: ", ticket.car_type,
							", \n Тип погрузчика: ", ticket.shipment_type,
							", \n Вес (кг): ", ticket.weight,
							", \n Объем (м3): ", ticket.volume,
							", \n Макс.длина (м): ", ticket.length,
							", \n Комментарий: ", ticket.comments,
						) + "\n\n Предложение от перевозчика: " +
						fmt.Sprintln(
							"\n Тип рассчетов: ", typePrice,
							"\n Прайс: ", bid.Price,
						)

					buttonClient := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData("Выбрать исполнителем", fmt.Sprintf(`{"step":200, "data":"%d"}`, bid.BidID)),
						),
					)

					msgClient := tgbotapi.NewMessage(clientChatID, messageClient)
					msgClient.ReplyMarkup = buttonClient
					/*result, err := */ bot.Send(msgClient)
					if err == nil {
						//messageID := result.MessageID
						//fmt.Println(messageID)
						//bot.Send(tgbotapi.NewEditMessageText(-1001370763028, messageID, "Test edit ok!"))
					}

					message = "Я отправил уведомление заказчику. Если его заинтересует Ваше предложение, он свяжется с Вами по номеру телефона, который Вы указали при регистрации"
				}
			}
		}

	case 106:
		_, err := db.Exec("update users set status=? where chat_id=?", step+1, chatID)
		if err != nil {
			log.Panic(err)
		}

		bids, err := db.Query("select ticket_id, type_price, price, bid_id from bid where status=4 and performer_id=?", chatID)
		if err != nil {
			log.Panic(err)
		}
		defer bids.Close()

		bidsRows := make([]*Bid, 0)

		for bids.Next() {
			bid := new(Bid)
			err = bids.Scan(&bid.TicketID, &bid.TypePrice, &bid.Price, &bid.BidID)
			if err != nil {
				log.Panic(err)
			}
			bidsRows = append(bidsRows, bid)
		}

		if len(bidsRows) > 0 {
			for _, bid := range bidsRows {
				tickets, err := db.Query("select customer_id, ticket_id, date, address_to, address_from, car_type, shipment_type, weight, volume, length, comments from tickets where ticket_id=?", bid.TicketID)
				if err != nil {
					log.Panic(err)
				}
				defer tickets.Close()

				ticketsRows := make([]*Ticket, 0)

				for tickets.Next() {
					ticket := new(Ticket)
					err = tickets.Scan(&ticket.customer_id, &ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.comments)
					if err != nil {
						log.Panic(err)
					}
					ticketsRows = append(ticketsRows, ticket)
				}

				if len(ticketsRows) > 0 {
					for _, ticket := range ticketsRows {
						clientChatID := int64(ticket.customer_id)
						typePrice := "почасовой тариф (руб./час)"

						if bid.TypePrice == 1 {
							typePrice = "фиксированная стоимость"
						}
						messageExecutor := "Заявка на исполнении: " +
							fmt.Sprintln(
								"№", ticket.ticket_id,
								", \n Дата и время: ", ticket.date,
								", \n Адрес погрузки: ", ticket.address_to,
								", \n Адрес выгрузки: ", ticket.address_from,
								", \n Тип автомобиля: ", ticket.car_type,
								", \n Тип погрузчика: ", ticket.shipment_type,
								", \n Вес (кг): ", ticket.weight,
								", \n Объем (м3): ", ticket.volume,
								", \n Макс.длина (м): ", ticket.length,
								", \n Комментарий: ", ticket.comments,
							) + "\n " +
							fmt.Sprintln(
								"\n Тип рассчетов: ", typePrice,
								"\n Прайс: ", bid.Price,
							)

						msgExecutor := tgbotapi.NewMessage(clientChatID, messageExecutor)
						//TODO: добавить возможность отмечать заявку выполненной, а заявителю подтверждать это или отправить жалобу
						/*msgExecutor.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData("Отметить как выполненную", fmt.Sprintf(`{"step":201, data":"%d"}`, ticket.ticket_id)),
							),
						)*/
						bot.Send(msgExecutor)
					}
				}
			}
		} else {
			message = "У Вас еще нет подтвержденных заявок на исполнении."
		}
	}

	msg = tgbotapi.NewMessage(chatID, message)

	if button.Keyboard != nil {
		msg.ReplyMarkup = button
	}
	if buttonInline.InlineKeyboard != nil {
		msg.ReplyMarkup = buttonInline
	}

	return msg
}

func ticketHandlerClientAndExecutant(step int, data string) (err error) {
	var messageClient string
	var messageExecutor string
	var messageExt string

	bot := botImplement()

	// выборка данных из БД для отправки уведомлений и обновления статусов
	bids, err := db.Query("select ticket_id, type_price, price, performer_id from bid where bid_id=?", data)
	if err != nil {
		log.Panic(err)
	}
	defer bids.Close()

	if bids.Next() {
		var bid Bid
		err = bids.Scan(&bid.TicketID, &bid.TypePrice, &bid.Price, &bid.PerformerID)
		if err != nil {
			log.Panic(err)
		}

		users, err := db.Query("select name, phone from users where chat_id=?", bid.PerformerID)
		if err != nil {
			log.Panic(err)
		}
		defer users.Close()

		if users.Next() {
			var user User
			err = users.Scan(&user.name, &user.phone)
			if err != nil {
				log.Panic(err)
			}

			// уведомление заказчику
			messageClient = fmt.Sprint("Вы приняли предложение. Контакты исполнителя: " +
				"\n Имя - " + user.name +
				"\n Телефон - " + user.phone)
		}

		tickets, err := db.Query("select customer_id, date, address_to, address_from, car_type, shipment_type, weight, volume, length, comments, chanel_message_id from tickets where ticket_id=?", bid.TicketID)
		if err != nil {
			log.Panic(err)
		}
		defer tickets.Close()

		if tickets.Next() != false {
			var ticket Ticket
			err = tickets.Scan(&ticket.customer_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.comments, &ticket.chanel_message_id)
			if err != nil {
				log.Panic(err)
			}

			executorID := int64(bid.PerformerID)
			clientChatID := int64(ticket.customer_id)

			switch step {
			case 200:
				// Перевозчик откликнулся на заявку, уведомляем все стороны
				// меняем статус отклика = 4
				_, err = db.Exec("update bid set status=4 where bid_id=?", data)
				if err != nil {
					log.Panic(err)
				}

				// меняем статус заявки = 3
				_, err = db.Exec("update tickets set status=3 where ticket_id=?", bid.TicketID)
				if err != nil {
					log.Panic(err)
				}

				// меняем статусы юзеров на 1
				_, err = db.Exec("update users set status=1 where chat_id=?", bid.PerformerID)
				if err != nil {
					log.Panic(err)
				}
				_, err = db.Exec("update users set status=1 where chat_id=?", ticket.customer_id)
				if err != nil {
					log.Panic(err)
				}

				typePrice := "почасовой тариф (руб./час)"

				if bid.TypePrice == 1 {
					typePrice = "фиксированная стоимость"
				}

				// уведомление исполнителю
				messageExecutor = "Вы выбраны исполнителем заявки: " +
					fmt.Sprintln(
						"№", bid.TicketID,
						", \n Дата и время: ", ticket.date,
						", \n Адрес погрузки: ", ticket.address_to,
						", \n Адрес выгрузки: ", ticket.address_from,
						", \n Тип автомобиля: ", ticket.car_type,
						", \n Тип погрузчика: ", ticket.shipment_type,
						", \n Вес (кг): ", ticket.weight,
						", \n Объем (м3): ", ticket.volume,
						", \n Макс.длина (м): ", ticket.length,
						", \n Комментарий: ", ticket.comments,
					) + "\n\n Предложение от перевозчика: " +
					fmt.Sprintln(
						"\n Тип рассчетов: ", typePrice,
						"\n Прайс: ", bid.Price,
					)

				messageExt = "Исполнитель уже выбран"
			case 201:
				// Заявитель отметил заявку выполненной, уведомляем исполнителя
				// меняем статус заявки = 4
				_, err = db.Exec("update tickets set status=4 where ticket_id=?", bid.TicketID)
				if err != nil {
					log.Panic(err)
				}

				// меняем статус отклика = 5
				_, err = db.Exec("update bid set status=5 where bid_id=?", data)
				if err != nil {
					log.Panic(err)
				}

				messageExt = "заявка завершена"
				messageClient = fmt.Sprintln(bid.TicketID, " ", messageExt)
				messageExecutor = messageClient
			}

			msgClient := tgbotapi.NewMessage(clientChatID, messageClient)
			msgExecutor := tgbotapi.NewMessage(executorID, messageExecutor)
			extMsg := tgbotapi.NewEditMessageReplyMarkup(-1001370763028, ticket.chanel_message_id, tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(messageExt, "done"))))

			_, err = bot.Send(msgClient)
			_, err = bot.Send(msgExecutor)
			_, err = bot.Send(extMsg)
		}
	}

	return err
}

func reportHandler(step int, data string, chatID int64) (msg tgbotapi.MessageConfig) {
	var message string
	var button tgbotapi.ReplyKeyboardMarkup
	var buttonInline tgbotapi.InlineKeyboardMarkup

	bot := botImplement()

	switch step {
	case 202:
		message = "Укажите дату начала периода для выгрузки (например, 2019-06-12)"
		_, err := db.Exec("update users set status=? where chat_id=?", (step + 1), chatID)
		if err != nil {
			log.Panic(err)
		}
	case 203:
		message = "Укажите дату конца периода для выгрузки (например, 2019-06-30)"
		layout := "2006-01-02"
		_, err := time.Parse(layout, data)
		if err == nil {
			pathName := fmt.Sprintln("reports/", chatID)
			os.RemoveAll(pathName)
			os.Mkdir(pathName, 0775)
			reportFile, err := os.Create(pathName + "/" + data)
			if err != nil {
				log.Panic(err)
				os.Exit(1)
			}
			defer reportFile.Close()

			_, err = db.Exec("update users set status=? where chat_id=?", (step + 1), chatID)
			if err != nil {
				log.Panic(err)
			}
		} else {
			message = "Укажите дату в формате ГГГГ-ММ-ДД, например, 2019-06-12"
		}
	case 204:
		role := 0
		users, err := db.Query("select role from users where chat_id=?", chatID)
		if err != nil {
			log.Panic(err)
		}
		defer users.Close()

		if users.Next() != false {
			var user User
			err = users.Scan(&user.role)
			if err != nil {
				log.Panic(err)
			}
			role = user.role
		}

		pathName := fmt.Sprintln("reports/", chatID)
		files, err := ioutil.ReadDir(pathName)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			dateStartStr := file.Name()
			fileName := dateStartStr + "_" + data + ".csv"
			err = os.Rename(pathName+"/"+file.Name(), pathName+"/"+fileName)

			if err == nil {
				fileOpen, err := os.OpenFile(pathName+"/"+fileName, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
				if err != nil {
					log.Panic(err)
					os.Exit(1)
				}
				defer fileOpen.Close()

				_, err = fileOpen.WriteString("№;Дата и время;Адрес погрузки;Адрес выгрузки;Тип автомобиля;Тип погрузчика;Вес (кг);Объем (м3);Макс.длина (м);Комментарий;Тип рассчетов;Прайс\n")
				if err != nil {
					log.Panic(err)
				}
				dateStart, _ := time.Parse("2006-01-02", dateStartStr)
				dateEnd, _ := time.Parse("2006-01-02 15:04", data+" 23:59")

				if role == 0 {
					tickets, err := db.Query("select customer_id, ticket_id, date, address_to, address_from, car_type, shipment_type, weight, volume, length, comments from tickets where customer_id = ? and date between ? and ?", chatID, dateStart, dateEnd)
					if err != nil {
						log.Panic(err)
					}
					defer tickets.Close()

					ticketsRows := make([]*Ticket, 0)
					for tickets.Next() {
						ticket := new(Ticket)
						err = tickets.Scan(&ticket.customer_id, &ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.comments)
						if err != nil {
							log.Panic(err)
						}
						ticketsRows = append(ticketsRows, ticket)
					}

					if len(ticketsRows) > 0 {
						for _, ticket := range ticketsRows {
							bids, err := db.Query("select ticket_id, type_price, price, bid_id from bid where status=4 and ticket_id=?", ticket.ticket_id)
							if err != nil {
								log.Panic(err)
							}
							defer bids.Close()

							bidsRows := make([]*Bid, 0)

							for bids.Next() {
								bid := new(Bid)
								err = bids.Scan(&bid.TicketID, &bid.TypePrice, &bid.Price, &bid.BidID)
								if err != nil {
									log.Panic(err)
								}
								bidsRows = append(bidsRows, bid)
							}

							if len(bidsRows) > 0 {
								for _, bid := range bidsRows {
									typePrice := "почасовой тариф (руб./час)"

									if bid.TypePrice == 1 {
										typePrice = "фиксированная стоимость"
									}
									item := fmt.Sprintln(
										ticket.ticket_id,
										";", ticket.date,
										";", ticket.address_to,
										";", ticket.address_from,
										";", ticket.car_type,
										";", ticket.shipment_type,
										";", ticket.weight,
										";", ticket.volume,
										";", ticket.length,
										";", ticket.comments,
										";", typePrice,
										";", bid.Price,
									)

									fileOpen.WriteString(item)
								}
							}
						}
					}

				} else {
					bids, err := db.Query("select ticket_id, type_price, price, bid_id from bid where status=4 and performer_id=?", chatID)
					if err != nil {
						log.Panic(err)
					}
					defer bids.Close()

					bidsRows := make([]*Bid, 0)

					for bids.Next() {
						bid := new(Bid)
						err = bids.Scan(&bid.TicketID, &bid.TypePrice, &bid.Price, &bid.BidID)
						if err != nil {
							log.Panic(err)
						}
						bidsRows = append(bidsRows, bid)
					}

					if len(bidsRows) > 0 {
						for _, bid := range bidsRows {
							tickets, err := db.Query("select customer_id, ticket_id, date, address_to, address_from, car_type, shipment_type, weight, volume, length, comments from tickets where ticket_id=? and date between ? and ?", bid.TicketID, dateStart, dateEnd)
							if err != nil {
								log.Panic(err)
							}
							defer tickets.Close()

							ticketsRows := make([]*Ticket, 0)

							for tickets.Next() {
								ticket := new(Ticket)
								err = tickets.Scan(&ticket.customer_id, &ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.comments)
								if err != nil {
									log.Panic(err)
								}
								ticketsRows = append(ticketsRows, ticket)
							}

							if len(ticketsRows) > 0 {
								for _, ticket := range ticketsRows {
									typePrice := "почасовой тариф (руб./час)"

									if bid.TypePrice == 1 {
										typePrice = "фиксированная стоимость"
									}
									item := fmt.Sprintln(
										ticket.ticket_id,
										";", ticket.date,
										";", ticket.address_to,
										";", ticket.address_from,
										";", ticket.car_type,
										";", ticket.shipment_type,
										";", ticket.weight,
										";", ticket.volume,
										";", ticket.length,
										";", ticket.comments,
										";", typePrice,
										";", bid.Price,
									)

									fileOpen.WriteString(item)
								}
							}
						}
					}
				}

			}
			message = "Отчет сформирован."
			msgFile := tgbotapi.NewDocumentUpload(chatID, pathName+"/"+fileName)
			bot.Send(msgFile)
		}
	}

	msg = tgbotapi.NewMessage(chatID, message)

	if button.Keyboard != nil {
		msg.ReplyMarkup = button
	}
	if buttonInline.InlineKeyboard != nil {
		msg.ReplyMarkup = buttonInline
	}

	return msg
}

func main() {
	GetConfig() // подключение конфига
	ConnectDB() // подключение к БД

	// Oбработчик запросов от telegram API
	bot := botImplement()
	bot.Debug = configuration.TelegramDebugMode
	if bot.Debug == true {
		log.Printf("Authorized on account %s", bot.Self.UserName)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// тип взаимодействия с API (переключается в config.json: "TelegramAPIMode" : "channel" или "webhook")
	updates := bot.ListenForWebhook("/" + bot.Token)
	if configuration.TelegramAPIMode == "channel" {
		updates, _ = bot.GetUpdatesChan(u)
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
				msg = Commands(chatID, userName, update.Message.Command(), update.Message.Text)
			} else {
				switch update.Message.Text {
				case "Помощь":
					msg = Commands(chatID, userName, "help", "")
				case "Создать заявку":
					msg = ticketHandlerClient(2, "", chatID)
				case "История заявок":
					msg = ticketHandlerClient(16, "", chatID)
				case "Скачать отчет (заказчик)":
					msg = reportHandler(202, "", chatID)
				case "Изменить роль":
					msg = changeRole(chatID)
				case "Все новые заявки":
					msg = ticketHandlerExecutant(101, "0", chatID)
				case "Исполняемые мной":
					msg = ticketHandlerExecutant(106, "0", chatID)
				case "Скачать отчет (перевозчик)":
					msg = reportHandler(202, "", chatID)
				default:
					step := getStep(chatID)
					if step < 100 {
						msg = ticketHandlerClient(step, update.Message.Text, chatID)
					} else if step >= 202 && step < 205 {
						msg = reportHandler(step, update.Message.Text, chatID)
					} else {
						msg = ticketHandlerExecutant(step, update.Message.Text, chatID)
					}
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
				msg = changeRole(chatID)
			}

			// Обработка остальных текстовых сообщений
		} else if update.CallbackQuery != nil {
			chatID := update.CallbackQuery.Message.Chat.ID

			dataMap := stepData{}
			json.Unmarshal([]byte(update.CallbackQuery.Data), &dataMap)

			step := dataMap.Step
			data := dataMap.Data
			role := dataMap.Role

			if role == 0 && step < 100 {
				msg = ticketHandlerClient(step, data, chatID)
				// Отправляем сообщение о новой заявке в общий канал
				if step == 12 {
					tickets := getTicketInfo(chatID, data, 1)
					defer tickets.Close()

					ticket := new(Ticket)
					err := tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address_to, &ticket.address_from, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.chanel_message_id)
					if err != nil {
						log.Panic(err)
					}
					message := "Новая заявка: \n" +
						fmt.Sprintln("№", ticket.ticket_id,
							", \nДата и время: ", ticket.date,
							", \nАдрес погрузки: ", ticket.address_to,
							", \nАдрес выгрузки: ", ticket.address_from,
							", \nТип автомобиля: ", ticket.car_type,
							", \nТип погрузчика: ", ticket.shipment_type,
							", \nВес (кг): ", ticket.weight,
							", \nОбъем (м3): ", ticket.volume,
							", \nМакс.длина (м): ", ticket.length,
							", \nКомментарий: ", ticket.comments)

					extMsg := tgbotapi.NewMessage(-1001370763028, message)
					bidButton := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonURL("Предложить цену", "http://t.me/devnil_bot?start="+data),
						),
					)
					extMsg.ReplyMarkup = bidButton
					result, err := bot.Send(extMsg)
					if err == nil && result.MessageID > 0 {
						_, err := db.Exec("update tickets set chanel_message_id=? where ticket_id=?", result.MessageID, ticket.ticket_id)
						if err != nil {
							log.Panic(err)
						}
					}
				}
			} else if step >= 100 && step < 200 {
				msg = ticketHandlerExecutant(step, data, chatID)
			} else if step >= 202 && step < 205 {
				msg = reportHandler(step, data, chatID)
			} else {
				err := ticketHandlerClientAndExecutant(step, data)
				if err != nil {
					log.Panic(err)
				}
			}

		}

		bot.Send(msg)
	}
}
