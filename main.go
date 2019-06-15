package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin/binding"
	"github.com/streadway/amqp"
)

var servicePort string

func setUpGETs(r *gin.Engine) {
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
}

type LOGIN struct {
	USER     string `json:"user" binding:"required"`
	PASSWORD string `json:"password" binding:"required"`
}

func setUpPOSTs(r *gin.Engine) {
	r.POST("/login", func(c *gin.Context) {
		var login LOGIN
		c.BindJSON(&login)
		fmt.Printf("login data : %v\n", login)
		c.JSON(200, gin.H{"status": login.USER}) //???
	})

}

func setUpPUTs(r *gin.Engine) {

}

func setUpDELETEs(r *gin.Engine) {

}

func setUpAPIs(r *gin.Engine) {

	setUpGETs(r)
	setUpPOSTs(r)
	setUpPUTs(r)
	setUpDELETEs(r)

}

func serviceUp(r *gin.Engine, port string) *http.Server {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	go func() {
		log.Println("starting server on port :" + port)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s. Shutting down...\n", err)
			panic(err)
		}
	}()
	return srv
}

func shutDownHTTPServer(srv *http.Server) {
	log.Println("Shutdown HTTP Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//Handle all graceful things
	if srv != nil {
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server Shutdown: ", err)
		}
	}
}

func closeRMQConnection(conn *amqp.Connection) {
	if conn != nil && !conn.IsClosed() {
		log.Println("Closing RMQ Connection...")
		err := conn.Close()
		failOnError(err, "RMQ Connection close")
	}
}

func handleSignal(srv *http.Server, conn *amqp.Connection) {
	// Setting up signal capturing
	log.Println("Handle signal...")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	// Waiting for SIGINT (pkill -2)
	<-stop
	shutDownHTTPServer(srv)
	closeRMQConnection(conn)
	log.Println("Server exiting")
}

func bringUpRMQConsumer(ch *amqp.Channel, queueName string) {
	msgs, err := ch.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			ch.Ack(d.DeliveryTag, false)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func declareQueue(ch *amqp.Channel, queueName string, rk string, exchName string) *amqp.Queue {
	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable, should the message be persistent? queue will survive if the cluster gets reset
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")
	bindQueue(ch, queueName, rk, exchName)
	return &q
}

func bindQueue(ch *amqp.Channel, queueName string, rk string, exchName string) {
	err := ch.QueueBind(
		queueName, // queue name
		rk,        // routing key
		exchName,  // exchange
		false,
		nil)
	failOnError(err, "Failed to bind a queue")
}

func bringUpRMQProducer(ch *amqp.Channel, exchName string, rk string, body string) {

	err := ch.Publish(
		exchName, // exchange
		rk,       // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	failOnError(err, "Failed to publish a message")
}

func setUpREST() *gin.Engine {
	r := gin.Default()
	setUpAPIs(r)
	return r
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func declareExchange(ch *amqp.Channel, exchName string, exchType string) {
	err := ch.ExchangeDeclare(
		exchName, // name
		exchType, // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")
}

// Reverse returns its argument string reversed rune-wise left to right.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func main() {
	Reverse("siddhartha")
	exchName := "sid_exchange"
	rk := "sid_rk"
	queueName := "sid_queue"
	exchType := "direct"
	port := "8089"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	r := setUpREST()
	srv := serviceUp(r, port)
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	ch, err := conn.Channel()
	defer ch.Close()
	failOnError(err, "Failed to open a channel")
	declareExchange(ch, exchName, exchType)
	declareQueue(ch, queueName, rk, exchName)
	go bringUpRMQConsumer(ch, queueName)
	go bringUpRMQProducer(ch, exchName, rk, "{\"key\":\"value\"}")
	handleSignal(srv, conn)
}
