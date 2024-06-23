package rabbitmq

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client is the base struct for handling connection recovery, consumption and
// publishing. Note that this struct has an internal mutex to safeguard against
// data races. As you develop and iterate over this example, you may need to add
// further locks, or safeguards, to keep your application safe from data races
type Client struct {
	m               *sync.Mutex
	queueName       string
	infolog         *log.Logger
	errlog          *log.Logger
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	notifyConfirm   chan amqp.Confirmation
	isReady         bool
	keys            []string // 队列绑定键
}

const (
	// When reconnecting to the server after connection failure
	reconnectDelay = 5 * time.Second

	// When setting up the channel after a channel exception
	reInitDelay = 2 * time.Second

	// When resending messages the server didn't confirm
	resendDelay = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("client is shutting down")
)

// New creates a new consumer state instance, and automatically
// attempts to connect to the server.
func New(queueName, addr string, keys []string) *Client {
	client := Client{
		m:         &sync.Mutex{},
		infolog:   log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lmsgprefix),
		errlog:    log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix),
		queueName: queueName,
		done:      make(chan bool),
		keys:      keys,
	}
	go client.handleReconnect(addr)
	return &client
}

// handleReconnect will wait for a connection error on
// notifyConnClose, and then continuously attempt to reconnect.
func (client *Client) handleReconnect(addr string) {
	for {
		client.m.Lock()
		client.isReady = false
		client.m.Unlock()

		client.infolog.Println("attempting to connect")

		conn, err := client.connect(addr)
		if err != nil {
			client.errlog.Println("failed to connect. Retrying...")

			select {
			case <-client.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := client.handleReInit(conn); done {
			break
		}
	}
}

// connect will create a new AMQP connection
func (client *Client) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)
	if err != nil {
		return nil, err
	}

	client.changeConnection(conn)
	client.infolog.Println("connected")
	return conn, nil
}

// handleReInit will wait for a channel error
// and then continuously attempt to re-initialize both channels
func (client *Client) handleReInit(conn *amqp.Connection) bool {
	for {
		client.m.Lock()
		client.isReady = false
		client.m.Unlock()

		err := client.init(conn)
		if err != nil {
			client.errlog.Println("failed to initialize channel, retrying...")

			select {
			case <-client.done:
				return true
			case <-client.notifyConnClose:
				client.infolog.Println("connection closed, reconnecting...")
				return false
			case <-time.After(reInitDelay):
			}
			continue
		}

		select {
		case <-client.done:
			return true
		case <-client.notifyConnClose:
			client.infolog.Println("connection closed, reconnecting...")
			return false
		case <-client.notifyChanClose:
			client.infolog.Println("channel closed, re-running init...")
		}
	}
}

// init will initialize channel & declare queue
func (client *Client) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Confirm(false)
	if err != nil {
		return err
	}
	_, err = ch.QueueDeclare(
		client.queueName,
		true,  // Durable
		false, // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // Arguments
	)
	if err != nil {
		return err
	}

	// 队列绑定路由键到amq.topic
	for _, v := range client.keys {
		if err = ch.QueueBind(
			client.queueName,
			v,           // Routing key
			"amq.topic", // Exchange
			false,       // No-wait
			nil,         // Arguments
		); err != nil {
			return err
		}
	}

	client.changeChannel(ch)
	client.m.Lock()
	client.isReady = true
	client.m.Unlock()
	client.infolog.Println("client init done")

	return nil
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (client *Client) changeConnection(connection *amqp.Connection) {
	client.connection = connection
	client.notifyConnClose = make(chan *amqp.Error, 1)
	client.connection.NotifyClose(client.notifyConnClose)
}

// changeChannel takes a new channel to the queue,
// and updates the channel listeners to reflect this.
func (client *Client) changeChannel(channel *amqp.Channel) {
	client.channel = channel
	client.notifyChanClose = make(chan *amqp.Error, 1)
	client.notifyConfirm = make(chan amqp.Confirmation, 1)
	client.channel.NotifyClose(client.notifyChanClose)
	client.channel.NotifyPublish(client.notifyConfirm)
}

// Push will push data onto the queue, and wait for a confirmation.
// This will block until the server sends a confirmation. Errors are
// only returned if the push action itself fails, see UnsafePush.
func (client *Client) Push(key string, data []byte) error {
	client.m.Lock()
	if !client.isReady {
		client.m.Unlock()
		return errors.New("failed to push: not connected")
	}
	client.m.Unlock()
	for {
		err := client.UnsafePush(key, data)
		if err != nil {
			client.errlog.Println("push failed. Retrying...")
			select {
			case <-client.done:
				return errShutdown
			case <-time.After(resendDelay):
			}
			continue
		}
		confirm := <-client.notifyConfirm
		if confirm.Ack {
			client.infolog.Printf("push confirmed [%d]", confirm.DeliveryTag)
			return nil
		}
	}
}

// UnsafePush will push to the queue without checking for
// confirmation. It returns an error if it fails to connect.
// No guarantees are provided for whether the server will
// receive the message.
func (client *Client) UnsafePush(key string, data []byte) error {
	client.m.Lock()
	if !client.isReady {
		client.m.Unlock()
		return errNotConnected
	}
	client.m.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 发布指定路由键到amq.topic
	return client.channel.PublishWithContext(
		ctx,
		"amq.topic", // Exchange
		key,         // Routing key
		false,       // Mandatory
		false,       // Immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         data,
		},
	)
}

// Consume will continuously put queue items on the channel.
// It is required to call delivery.Ack when it has been
// successfully processed, or delivery.Nack when it fails.
// Ignoring this will cause data to build up on the server.
func (client *Client) Consume() (<-chan amqp.Delivery, error) {
	client.m.Lock()
	if !client.isReady {
		client.m.Unlock()
		return nil, errNotConnected
	}
	client.m.Unlock()

	if err := client.channel.Qos(
		1,     // prefetchCount
		0,     // prefetchSize
		false, // global
	); err != nil {
		return nil, err
	}

	return client.channel.Consume(
		client.queueName,
		"",    // Consumer
		false, // Auto-Ack
		false, // Exclusive
		false, // No-local
		false, // No-Wait
		nil,   // Args
	)
}

// Close will cleanly shut down the channel and connection.
func (client *Client) Close() error {
	client.m.Lock()
	// we read and write isReady in two locations, so we grab the lock and hold onto
	// it until we are finished
	defer client.m.Unlock()

	if !client.isReady {
		return errAlreadyClosed
	}
	close(client.done)
	err := client.channel.Close()
	if err != nil {
		return err
	}
	err = client.connection.Close()
	if err != nil {
		return err
	}

	client.isReady = false
	return nil
}
