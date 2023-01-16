package main

import (
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/nats.go"
)

func main() {
	port := "54222"
	context := ""
	bucketName := "terraform-state"
	maxBytes := 1024 * 1024 * 512

	nc, err := natscontext.Connect(context)
	if err != nil {
		log.Fatalln(err)
	}

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalln(err)
	}

	objectStore, err := js.CreateObjectStore(&nats.ObjectStoreConfig{
		Bucket:      bucketName,
		Description: "Stores terraform state",
		MaxBytes:    int64(maxBytes),
	})
	if err != nil {
		log.Fatalln(err)
	}

	r := chi.NewRouter()

	r.Route("/state/{name}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, "name")

			bytes, err := objectStore.GetBytes(name)
			if err != nil {
				log.Println("Error", err)
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			w.Write(bytes)
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, "name")

			bytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("Error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			_, err = objectStore.PutBytes(name, bytes)
			if err != nil {
				log.Println("Error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, "name")

			err = objectStore.Delete(name)
			if err != nil {
				log.Println("Error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	})

	log.Println("Listening on port", port)
	http.ListenAndServe(":"+port, r)
}
