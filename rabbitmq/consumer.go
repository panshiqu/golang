package rabbitmq

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 连接通道关闭时重新消费延迟
const reConsumeDelay = 5 * time.Second

func (queue *Client) ConsumeFunc(ctx context.Context, fn func(*amqp.Delivery) error) {
	// This channel will receive a notification when a channel closed event
	// happens. This must be different from Client.notifyChanClose because the
	// library sends only one notification and Client.notifyChanClose already has
	// a receiver in handleReconnect().
	// Recommended to make it buffered to avoid deadlocks
	chClosedCh := make(chan *amqp.Error, 1)
	deliveries, err := queue.Consume()
	if err != nil {
		queue.errlog.Printf("could not start consuming: %s\n", err)
		// 支持程序先于RabbitMQ启动
		close(chClosedCh)
	} else {
		queue.channel.NotifyClose(chClosedCh)
	}

	for {
		select {
		case <-ctx.Done():
			err := queue.Close()
			if err != nil {
				queue.errlog.Printf("close failed: %s\n", err)
			}
			return

		case amqErr := <-chClosedCh:
			// This case handles the event of closed channel e.g. abnormal shutdown
			queue.errlog.Printf("AMQP Channel closed due to: %s\n", amqErr)

			deliveries, err = queue.Consume()
			if err != nil {
				// If the AMQP channel is not ready, it will continue the loop. Next
				// iteration will enter this case because chClosedCh is closed by the
				// library
				queue.errlog.Println("error trying to consume, will try again")
				select {
				case <-time.After(reConsumeDelay):
				// 程序退出时提前结束延迟
				case <-ctx.Done():
				}
				continue
			}

			// Re-set channel to receive notifications
			// The library closes this channel after abnormal shutdown
			chClosedCh = make(chan *amqp.Error, 1)
			queue.channel.NotifyClose(chClosedCh)

		case delivery := <-deliveries:
			queue.infolog.Printf("received message: %s\n", delivery.Body)
			if err := delivery.Ack(false); err != nil {
				queue.errlog.Printf("error acknowledging message: %s\n", err)
			}
		}
	}
}
