package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/valyala/fasthttp"
)

// Coord is a Lat Long struct.
type Coord2D struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Coord3D struct {
	Coord2D
	Alt float64 `json:"alt"`
}

type Coord4D struct {
	Coord3D
	Timestamp int64 `json:"timestamp"`
}

type session struct {
	val          float64
	stateChannel chan Coord4D
}

type sessionsLock struct {
	MU       sync.Mutex
	sessions []*session
}

func (sl *sessionsLock) addSession(s *session) {
	sl.MU.Lock()
	sl.sessions = append(sl.sessions, s)
	sl.MU.Unlock()
}

func (sl *sessionsLock) removeSession(s *session) {
	sl.MU.Lock()
	idx := slices.Index(sl.sessions, s)
	if idx != -1 {
		sl.sessions[idx] = nil
		sl.sessions = slices.Delete(sl.sessions, idx, idx+1)
	}
	sl.MU.Unlock()
}

var currentSessions sessionsLock

var currentPosition *Coord4D

func formatSSEMessage(eventType string, data any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	m := map[string]any{
		"data": data,
	}

	err := enc.Encode(m)
	if err != nil {
		return "", nil
	}
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("event: %s\n", eventType))
	sb.WriteString(fmt.Sprintf("retry: %d\n", 15000))
	sb.WriteString(fmt.Sprintf("data: %v\n\n", buf.String()))

	return sb.String(), nil
}

func WrapAround(f, low, high float64) float64 {
	if f < low {
		return high
	}
	if f > high {
		return low
	}
	return f
}

func main() {

	// -90 to 90 for latitude and -180 to 180 for longitude

	pos := Coord3D{}
	pos.Lat = 51.477487
	pos.Lon = -0.341004
	pos.Alt = 1000

	currentPosition = &Coord4D{
		Coord3D:   pos,
		Timestamp: time.Now().Unix(),
	}

	app := fiber.New()

	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Send(nil)
	})

	app.Get("/sse", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		query := c.Query("query")

		log.Printf("New Request\n")

		stateChan := make(chan Coord4D)

		val, err := strconv.ParseFloat(query, 64)
		if err != nil {
			val = 0
		}

		s := session{
			val:          val,
			stateChannel: stateChan,
		}

		currentSessions.addSession(&s)

		notify := c.Context().Done()

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			keepAliveTickler := time.NewTicker(30 * time.Second)
			keepAliveMsg := ":keepalive\n"

			// listen to signal to close and unregister (doesn't seem to be called)
			go func() {
				<-notify
				log.Printf("Stopped Request\n")
				currentSessions.removeSession(&s)
				keepAliveTickler.Stop()
			}()

			for loop := true; loop; {
				select {

				case ev := <-stateChan:
					sseMessage, err := formatSSEMessage("current-value", ev)
					if err != nil {
						log.Printf("Error formatting sse message: %v\n", err)
						continue
					}

					// send sse formatted message
					_, err = fmt.Fprintf(w, sseMessage)

					if err != nil {
						log.Printf("Error while writing Data: %v\n", err)
						continue
					}

					err = w.Flush()
					if err != nil {
						log.Printf("Error while flushing Data: %v\n", err)
						currentSessions.removeSession(&s)
						keepAliveTickler.Stop()
						loop = false
						break
					}
				case <-keepAliveTickler.C:
					fmt.Fprintf(w, keepAliveMsg)
					err := w.Flush()
					if err != nil {
						log.Printf("Error while flushing: %v.\n", err)
						currentSessions.removeSession(&s)
						keepAliveTickler.Stop()
						loop = false
						break
					}
				}
			}

			log.Println("Exiting stream")
		}))

		return nil
	})

	ticker := time.NewTicker(25 * time.Millisecond)

	go func() {
		for {
			select {
			case <-ticker.C:
				// fmt.Println("Tick at", t)
				wg := &sync.WaitGroup{}

				// send a broadcast event, so all clients connected
				// will receive it, by filtering based on some info
				// stored in the session it is possible to address
				// only specific clients
				for _, s := range currentSessions.sessions {
					wg.Add(1)
					go func(cs *session) {
						defer wg.Done()
						currentPosition.Lat = WrapAround(currentPosition.Lat-0.08, -90, 90)
						currentPosition.Lon = WrapAround(currentPosition.Lon-0.08, -180, 180)
						currentPosition.Timestamp = time.Now().Unix()
						cs.stateChannel <- *currentPosition
					}(s)
				}
				wg.Wait()
			}
		}
	}()

	err := app.Listen("localhost:8080")
	if err != nil {
		log.Panic(err)
	}
}
