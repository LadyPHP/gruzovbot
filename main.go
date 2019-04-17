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
		_, err = db.Exec("update tickets set "+field+"=? where customer_id=? and status=0", updValue, chatID)
		if err != nil {
			log.Panic(err)
		}
	} else {
		updValue := value
		_, err := db.Exec("update tickets set "+field+"=? where customer_id=? and status=0", updValue, chatID)
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

	updates, err := bot.GetUpdatesChan(u)
	//updates := bot.ListenForWebhook("/" + bot.Token)

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

			if update.Message.IsCommand() { // если это команда
				switch update.Message.Command() {
				case "start":
					step = 0
				}
			}

			msgText := "Здравствуйте " + update.Message.Chat.FirstName + "! Я Ваш ассистент-бот."
			msg := tgbotapi.NewMessage(chatID, msgText)

			switch step {
			case 0:
				msgText = msgText + "Для начала работы отправьте ваш телефон (кнопка внизу)."
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
			default: // обработка дальнейших шагов по ролям
				if role == 0 {
					// Оформление заявки заказчиком
					switch step {
					case 3:
						switch update.Message.Text {
						case "Создать новую":
							msgText = "Время и дата погрузки"
							msg = tgbotapi.NewMessage(chatID, msgText)

							// создаем запись в БД о новой заявке
							_, err := db.Exec("insert into tickets (customer_id, status) values (?, 0)", chatID)
							if err != nil {
								log.Panic(err)
							}
							// обновляем шаг для пользователя
							_, err = db.Exec("update users set status=4 where chat_id=?", chatID)
							if err != nil {
								log.Panic(err)
							}
						case "История заявок":
							tickets, err := db.Query("select * from tickets where customer_id=?", chatID)
							if err != nil {
								log.Panic(err)
							}
							defer tickets.Close()

							if tickets.Next() != false {
								// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
								var ticket Ticket
								err = tickets.Scan(&ticket.ticket_id, &ticket.status, &ticket.address, &ticket.date, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length, &ticket.options, &ticket.customer_id)
								if err != nil {
									log.Panic(err)
								}
								msgText = msgText + fmt.Sprintf("№ %s, статус: %s, дата и время: %s, адрес: %s, комментарий: %s, тип автомобиля: %s, тип погрузки: %s, вес (кг): %s, объем (м3): %s, макс. длина (м): %s", ticket.ticket_id, ticket.status, ticket.date, ticket.address, ticket.comments, ticket.car_type, ticket.shipment_type, ticket.weight, ticket.volume, ticket.length)
							} else {
								// иначе показываем просто сообщение
								msgText = "Вы пока не создали ниодной заявки. Чтобы начать, нажмите кнопку \"Создать новую\""
							}
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 4:
						result := UpdateTicket(chatID, step, "date", update.Message.Text)
						if result == true {
							msgText = "Адрес погрузки/выгрузки"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 5:
						result := UpdateTicket(chatID, step, "address", update.Message.Text)
						if result == true {
							msgText = "Тип автомобиля (на выбор один или несколько): закрытый/открытый/рефрижератор"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 6:
						result := UpdateTicket(chatID, step, "car_type", update.Message.Text)
						if result == true {
							msgText = "Тип погрузки (на выбор один или несколько): верхняя/задняя/боковая"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 7:
						result := UpdateTicket(chatID, step, "shipment_type", update.Message.Text)
						if result == true {
							msgText = "Вес груза, кг"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 8:
						if err == nil {
							result := UpdateTicket(chatID, step, "weight", update.Message.Text)
							if result == true {
								msgText = "Объем груза, м3"
								msg = tgbotapi.NewMessage(chatID, msgText)
							}
						}
					case 9:
						result := UpdateTicket(chatID, step, "volume", update.Message.Text)
						if result == true {
							msgText = "Максимальная длина (если известна)"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 10:
						result := UpdateTicket(chatID, step, "length", update.Message.Text)
						if result == true {
							msgText = "Дополнительная информация"
							msg = tgbotapi.NewMessage(chatID, msgText)
						}
					case 11:
						result := UpdateTicket(chatID, step, "comments", update.Message.Text)
						if result == true {
							msgText = "Проверьте информацию и подтвердите публикацию заявки."
							tickets, err := db.Query("select ticket_id, date, address, comments, car_type, shipment_type, weight, volume, length from tickets where customer_id=?", chatID)
							if err != nil {
								log.Panic(err)
							}
							defer tickets.Close()

							if tickets.Next() != false {
								// если есть ранее созданные заказчиком заказы, выводим сообщением со ссылками
								var ticket Ticket
								err = tickets.Scan(&ticket.ticket_id, &ticket.date, &ticket.address, &ticket.comments, &ticket.car_type, &ticket.shipment_type, &ticket.weight, &ticket.volume, &ticket.length)
								if err != nil {
									log.Panic(err)
								}
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
							}
							// отпраляем сообщение
							msg = tgbotapi.NewMessage(chatID, msgText)
							// отправляем кнопки
							msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
								tgbotapi.NewInlineKeyboardRow(
									tgbotapi.NewInlineKeyboardButtonData("Опубликовать", "1"),
									tgbotapi.NewInlineKeyboardButtonData("Отменить", "0"),
								),
							)
						}
					}
				} else {
					// Обработка заявки исполнителем
					switch step {
					case 3:
						//
					}
				}

			}

			sm, _ := bot.Send(msg)
			lastID = sm.MessageID

		} else {
			if lastID != 0 && update.CallbackQuery != nil {
				step := 2
				role := uint64(0)
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
					role = uint64(user.role)
				}

				msgText := ""
				msg := tgbotapi.NewMessage(chatID, msgText)

				switch step {
				case 2:
					// запоминаем выбранную роль
					role, err = strconv.ParseUint(update.CallbackQuery.Message.Text, 0, 64)
					if err != nil {
						// не отлавливаем ошибку, а просто ставим роль = 0
						role = 0
					}

					_, err = db.Exec("update users set status=3, role=? where chat_id=?", role, chatID)
					if err != nil {
						log.Panic(err)
					}

					msgText = "Отлично, при необходимости изменить роль, просто воспользуйтесь соответствующим пунктом меню."
					msgText = msgText + "Для продолжения работы с заявками выберите в меню:"
					msg = tgbotapi.NewMessage(chatID, msgText)
					if role == 0 {
						msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
							tgbotapi.NewKeyboardButtonRow(
								tgbotapi.NewKeyboardButton("Создать новую"),
								tgbotapi.NewKeyboardButton("История заявок"),
							),
						)
					} else {
						msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
							tgbotapi.NewKeyboardButtonRow(
								tgbotapi.NewKeyboardButton("Все новые заявки"),
								tgbotapi.NewKeyboardButton("Мои заявки в работе"),
							),
						)
					}
				case 12:
					if role == 0 {
						fmt.Sprintln(update.CallbackQuery.Message.Text)
						published, err := strconv.ParseInt(update.CallbackQuery.Message.Text, 0, 64)
						if err != nil {
							if published == 1 {
								_, err := db.Exec("update tickets set status=1 where customer_id=? and status=0", chatID)
								if err != nil {
									log.Panic(err)
								}
								_, err = db.Exec("update users set status=13 where chat_id=?", role, chatID)
								if err != nil {
									log.Panic(err)
								}

								msgText = "Заявка опубликована. Ожидайте отклики от исполнителей."
							} else {
								msgText = "Готово"
							}
						}
					}
					msg = tgbotapi.NewMessage(chatID, msgText)
				}
				sm, _ := bot.Send(msg)
				lastID = sm.MessageID
			}
		}
	}
}
