package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/nats.go"
)

var (
	port        = os.Getenv("PORT")
	context     = os.Getenv("CONTEXT")
	bucket      = os.Getenv("BUCKET")
	maxBytesStr = os.Getenv("MAX_BYTES")
)

func main() {
	if port == "" {
		port = "54222"
	}

	if bucket == "" {
		bucket = "terraform-state"
	}

	maxBytes := 1024 * 1024 * 512
	if maxBytesStr != "" {
		var err error
		maxBytes, err = strconv.Atoi(maxBytesStr)
		if err != nil {
			log.Fatalln(err)
		}
	}

	nc, err := natscontext.Connect(context)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Connected to NATS server", nc.ConnectedUrl())

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalln(err)
	}

	objectStore, err := js.CreateObjectStore(&nats.ObjectStoreConfig{
		Bucket:      bucket,
		Description: "Stores terraform state",
		MaxBytes:    int64(maxBytes),
	})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Using JetStream object store bucket", bucket)

	locks, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:      bucket + "-locks",
		Description: "Stores locks for terraform state bucket " + bucket,
		MaxBytes:    1024 * 1024,
	})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Using JetStream kv", bucket+"-locks", "for locking")

	s := NewServer(objectStore, locks)

	http.Handle("/state/", http.StripPrefix("/state/", s))

	log.Println("Listening on port", port)
	http.ListenAndServe(":"+port, http.DefaultServeMux)
}
