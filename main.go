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
	"strings"
)

var db *sql.DB
var configuration Config
var InsertTicket int64

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

func GetTickets(chatID int64) Ticket {
	tickets, err := db.Query("select ticket_id, date, address, comments, car_type, shipment_type, weight, volume, length from tickets where customer_id=?", chatID)
	if err != nil {
		log.Panic(err)
	}
	defer tickets.Close()

	if tickets.Next() == false {
		log.Panic(err)
	}
	// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
	var ticket Ticket
	err = tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
	if err != nil {
		log.Panic(err)
	}
	return ticket
}

func MainMenu(chatID int64, role int64) (msg tgbotapi.ReplyKeyboardMarkup) {
	btnText1 := "Создать новую"
	btnText2 := "История заявок"

	if role == 1 {
		btnText1 = "Все новые заявки"
		btnText2 = "Исполняемые мной"
	}

	_, err := db.Exec("update users set status=1000 where chat_id=?", chatID)
	if err != nil {
		log.Panic(err)
	}
	//fmt.Println(chatID)

	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnText1),
			tgbotapi.NewKeyboardButton(btnText2),
			tgbotapi.NewKeyboardButton("Изменить роль"),
		),
	)
}

func CustomerBranch(chatID int64, step int, inMessage string) {
	bot, err := tgbotapi.NewBotAPI(configuration.TelegramBotToken)
	msgText := ""
	msg := tgbotapi.NewMessage(chatID, msgText)
	sendFlag := true // по умолчанию разрешаем отправку сообщений на каждом шаге

	switch step {
	case 4:
		result := UpdateTicket(chatID, step, "date", inMessage)
		if result == true {
			msgText = "Адрес погрузки/выгрузки"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 5:
		result := UpdateTicket(chatID, step, "address", inMessage)
		if result == true {
			msgText = "Тип автомобиля (на выбор один или несколько): закрытый/открытый/рефрижератор"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 6:
		result := UpdateTicket(chatID, step, "car_type", inMessage)
		if result == true {
			msgText = "Тип погрузки (на выбор один или несколько): верхняя/задняя/боковая"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 7:
		result := UpdateTicket(chatID, step, "shipment_type", inMessage)
		if result == true {
			msgText = "Вес груза, кг"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 8:
		if err == nil {
			result := UpdateTicket(chatID, step, "weight", inMessage)
			if result == true {
				msgText = "Объем груза, м3"
				msg = tgbotapi.NewMessage(chatID, msgText)
			}
		}
	case 9:
		result := UpdateTicket(chatID, step, "volume", inMessage)
		if result == true {
			msgText = "Максимальная длина, м (если известна)"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 10:
		result := UpdateTicket(chatID, step, "length", inMessage)
		if result == true {
			msgText = "Дополнительная информация"
			msg = tgbotapi.NewMessage(chatID, msgText)
		}
	case 11:
		result := UpdateTicket(chatID, step, "comments", inMessage)
		if result == true {
			msgText = "Проверьте информацию и подтвердите публикацию заявки."
			ticket := GetTickets(chatID)
			// обновляем шаг пользователя
			_, err = db.Exec("update users set status=? where chat_id=?", step+1, chatID)
			if err != nil {
				log.Panic(err)
			}
			// дополняем сообщение информацией о заявке
			msgText = msgText + fmt.Sprintln("№ + ", ticket.ticket_id,
				", Дата и время: ", ticket.date, ", Адрес: ", ticket.address,
				", Комментарий: ", ticket.comments, ", Тип автомобиля: ", ticket.car_type,
				", Тип погрузчика: ", ticket.shipment_type, ", Вес (кг): ", ticket.weight,
				", Объем (м3): ", ticket.volume, ", Макс.длина (м): ", ticket.length)

			// отпраляем сообщение
			msg = tgbotapi.NewMessage(chatID, msgText)
			// отправляем кнопки
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Опубликовать", "1"),
					tgbotapi.NewInlineKeyboardButtonData("Редактировать", "2"),
					tgbotapi.NewInlineKeyboardButtonData("Отменить", "0"),
				),
			)
		}
	default:
		switch inMessage {
		case "Создать новую":
			msgText = "Время и дата погрузки"
			msg = tgbotapi.NewMessage(chatID, msgText)

			// создаем запись в БД о новой заявке
			result, err := db.Exec("insert into tickets (customer_id, status) values (?, 1)", chatID)
			InsertTicket, err = result.LastInsertId()
			if err != nil {
				log.Panic(err)
			}
			// обновляем шаг для пользователя
			_, err = db.Exec("update users set status=4 where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
		case "История заявок":
			tickets, err := db.Query("select ticket_id, status, date, address, comments, car_type, shipment_type, weight, volume, length from tickets where customer_id=? order by ticket_id", chatID)
			if err != nil {
				log.Panic(err)
			}
			defer tickets.Close()

			ticketsRows := make([]*Ticket, 0)

			for tickets.Next() {
				// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
				ticket := new(Ticket)
				err = tickets.Scan(&ticket.ticket_id, &ticket.status, &ticket.date, &ticket.address, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
				if err != nil {
					log.Panic(err)
				}
				ticketsRows = append(ticketsRows, ticket)
			}

			if len(ticketsRows) > 0 {
				sendFlag = false

				for _, ticket := range ticketsRows {
					btnText1 := "Отменить"
					btnData1 := fmt.Sprintf("0 %d", ticket.ticket_id)
					btnText2 := "Копировать"
					btnData2 := fmt.Sprintf("1 %d", ticket.ticket_id)
					if ticket.status <= 1 {
						btnText1 = "Опубликовать"
						btnData1 = fmt.Sprintf("2 %d", ticket.ticket_id)
						btnText2 = "Изменить"
						btnData2 = fmt.Sprintf("3 %d", ticket.ticket_id)
					}
					msgText = fmt.Sprintln(
						"№ + ", ticket.ticket_id,
						", \n Дата и время: ", ticket.date,
						", \n Адрес: ", ticket.address,
						", \n Комментарий: ", ticket.comments,
						", \n Тип автомобиля: ", ticket.car_type,
						", \n Тип погрузчика: ", ticket.shipment_type,
						", \n Вес (кг): ", ticket.weight,
						", \n Объем (м3): ", ticket.volume,
						", \n Макс.длина (м): ", ticket.length,
					)

					msg = tgbotapi.NewMessage(chatID, msgText)
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData(btnText1, btnData1),
							tgbotapi.NewInlineKeyboardButtonData(btnText2, btnData2),
						),
					)
					bot.Send(msg)
				}
				// обновляем шаг для пользователя
				_, err := db.Exec("update users set status=12 where chat_id=?", chatID)
				if err != nil {
					log.Panic(err)
				}
			} else {
				msgText = "Вы пока не создали ниодной заявки. \n Чтобы начать, нажмите кнопку \"Создать новую\" в меню."
				msg = tgbotapi.NewMessage(chatID, msgText)
			}
		case "Изменить роль":
			msgText = "Выберите:"
			msg = tgbotapi.NewMessage(chatID, msgText)
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Заказчик", "0"),
					tgbotapi.NewInlineKeyboardButtonData("Перевозчик", "1"),
				),
			)
			_, err = db.Exec("update users set status=2 where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	if sendFlag == true {
		bot.Send(msg)
	}
}

//func PerformerBranch (chatID int64, step int, inMessage string) {}

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
		if update.Message != nil { // если текстовое сообщение
			chatID := update.Message.Chat.ID
			userName := update.Message.Chat.LastName + " " + update.Message.Chat.FirstName

			// Определяем есть ли пользователь в базе и на каком он шаге
			users, err := db.Query("select * from users where chat_id=?", chatID)
			if err != nil {
				log.Panic(err)
			}
			defer users.Close()

			// если нет, то добавляем
			step := int(0)
			role := 0
			if users.Next() == false {
				_, err := db.Exec("insert into users (chat_id, name, status) values (?, ?, ?)", chatID, userName, 0)
				if err != nil {
					log.Panic(err)
				}
			} else { // иначе вычисляем на каком он шаге и с какой ролью
				var user User
				err = users.Scan(&user.chat_id, &user.name, &user.phone, &user.status, &user.role)
				if err != nil {
					log.Panic(err)
				}
				step = user.status
				role = user.role
			}

			// Обработчик команд
			if update.Message.IsCommand() { // если это команда
				switch update.Message.Command() {
				case "start":
					step = 0
				}
			}

			msgText := ""
			msg := tgbotapi.NewMessage(chatID, msgText)

			// Обработчик по полученным состояниям (шагам)
			switch step {
			// приветсвие и запрос телефона
			case 0:
				msgText = "Здравствуйте " + update.Message.Chat.FirstName + "! \n Я Ваш ассистент-бот. Для начала работы отправьте ваш телефон (кнопка внизу)."
				msg = tgbotapi.NewMessage(chatID, msgText)
				msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButtonContact("Отправить телефон"),
					),
				)
				_, err := db.Exec("update users set status=1 where chat_id=?", chatID)
				if err != nil {
					log.Panic(err)
				}
			// запись телефона в базу
			case 1:
				if update.Message.Contact != nil {
					Phone, err := strconv.ParseUint(update.Message.Contact.PhoneNumber, 0, 64)
					if err == nil {
						_, err := db.Exec("update users set phone=?, status=2 where chat_id=?", Phone, chatID)
						if err != nil {
							log.Panic(err)
						}

						msgText = update.Message.Contact.PhoneNumber
					}
				}

				msgText = "Выберите:"
				msg = tgbotapi.NewMessage(chatID, msgText)
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Заказчик", "0"),
						tgbotapi.NewInlineKeyboardButtonData("Перевозчик", "1"),
					),
				)
			// обработка дальнейших шагов по ролям
			default:
				msg.ReplyMarkup = MainMenu(chatID, 0)

				if role == 0 {
					// Оформление заявки заказчиком
					CustomerBranch(chatID, step, update.Message.Text)
				} else {
					// Обработка заявки исполнителем
					switch step {
					case 3:
						//
					}
				}

			}

			bot.Send(msg)

		} else {
			if update.CallbackQuery != nil {
				step := 2
				role := int64(0)
				chatID := update.CallbackQuery.Message.Chat.ID
				// Определяем есть ли пользователь в базе и на каком он шаге
				users, err := db.Query("select * from users where chat_id=?", chatID)
				if err != nil {
					log.Panic(err)
				}
				defer users.Close()

				if users.Next() != false {
					var user User
					err = users.Scan(&user.chat_id, &user.name, &user.phone, &user.status, &user.role)
					if err != nil {
						log.Panic(err)
					}
					step = user.status
					role = int64(user.role)
				}

				msgText := ""
				msg := tgbotapi.NewMessage(chatID, msgText)
				//fmt.Println(msgText)

				switch step {
				case 2:
					// запоминаем выбранную роль
					role, err = strconv.ParseInt(update.CallbackQuery.Data, 0, 64)
					if err != nil {
						// не отлавливаем ошибку, а просто ставим роль = 0
						role = 0
					}

					msgText = "Отлично, при необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню.\n\n"
					msgText = msgText + "Для продолжения работы с заявками выберите в меню одно из действий."
					msg = tgbotapi.NewMessage(chatID, msgText)
					msg.ReplyMarkup = MainMenu(chatID, role)

					_, err = db.Exec("update users set status=3, role=? where chat_id=?", role, chatID)
					if err != nil {
						log.Panic(err)
					}
				case 12:
					ticketMsgArr := strings.Fields(update.CallbackQuery.Data)
					ticketID, errTicketNum := strconv.ParseInt(ticketMsgArr[1], 0, 64)
					ticketMenu := tgbotapi.NewInlineKeyboardMarkup()
					changeMenu := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Дата и время\n", "4")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Адрес погрузки/выгрузки\n", "5")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Тип автомобиля\n", "2")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Вес груза в кг\n", "3")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Объем груза в м3\n", "4")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Максимальная длина в м\n", "5")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Дополнительная информация", "6")),
						tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Опубликовать", "6")),
					)

					if role == 0 {
						action, err := strconv.ParseInt(ticketMsgArr[0], 0, 64)
						if err == nil && errTicketNum == nil {
							status := 12 // статус по умолчанию после нажатия кнопки одного из действий с заявкой
							switch action {
							case 1:
								_, err := db.Exec("insert into tickets (address, date, options, comments, status, customer_id, car_type, shipment_type, weight, volume, length) select address, date, options, comments, 1, customer_id, car_type, shipment_type, weight, volume, length from tickets where customer_id =? and ticket_id =?", chatID, ticketID)
								fmt.Println(chatID, ticketID)
								if err != nil {
									log.Panic(err)
								}
								status = 4
								msgText = "Заявка скопирована. Выберите параметр, который нужно изменить."
								ticketMenu = changeMenu
								status = 13
							case 2:
								_, err := db.Exec("update tickets set status=2 where customer_id=? and ticket_id=? and status<=1", chatID, ticketID)
								if err != nil {
									log.Panic(err)
								}
								status = 14
								msgText = "Заявка опубликована. Ожидайте отклики от исполнителей."
							case 3:
								msgText = "Выберите параметр, который нужно изменить."
								ticketMenu = changeMenu
								status = 13
							default:
								_, err := db.Exec("update tickets set status=0 where customer_id=? and ticket_id=?", chatID, ticketID)
								if err != nil {
									log.Panic(err)
								}
								msgText = "Заявка отменена."
							}
							fmt.Println(status)
							_, err = db.Exec("update users set status=? where chat_id=?", status, chatID)
							if err != nil {
								log.Panic(err)
							}
						}
					}
					msg = tgbotapi.NewMessage(chatID, msgText)
					if len(ticketMenu.InlineKeyboard) > 0 {
						msg.ReplyMarkup = ticketMenu
					}
				case 13:
					ticketMsgArr := strings.Fields(update.CallbackQuery.Data)
					action, err := strconv.ParseInt(ticketMsgArr[0], 0, 64)
					//ticketID, errTicketNum := strconv.ParseInt(ticketMsgArr[1], 0, 64)
					if err == nil {
						switch action {
						case 4:
							msgText = "Укажите новое значение - " + update.CallbackQuery.Message.Text
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
						/*CustomerBranch(chatID, int(action), update.Message.Text)
						_, err = db.Exec("update users set status=12 where chat_id=?", chatID)
						if err != nil {
							log.Panic(err)
						}*/
					}
				}

				bot.Send(msg)
			}
		}
	}
}
