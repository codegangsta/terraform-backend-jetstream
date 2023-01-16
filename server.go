package main

import (
	"io"
	"log"
	"net/http"

	"github.com/codegangsta/mixer"
	"github.com/nats-io/nats.go"
)

type Context struct {
	mixer.Context
	name string
}

func (c *Context) Error(err error) {
	log.Println("Error", err)
	http.Error(c.ResponseWriter(), err.Error(), http.StatusInternalServerError)
}

type Server struct {
	bucket   nats.ObjectStore
	locks    nats.KeyValue
	handlers map[string]http.Handler
}

func NewServer(bucket nats.ObjectStore, locks nats.KeyValue) *Server {
	s := &Server{
		bucket: bucket,
		locks:  locks,
	}

	m := mixer.New(func(ctx mixer.Context) *Context {
		return &Context{Context: ctx}
	})

	// Parse name
	m.Before(func(ctx *Context) {
		ctx.name = ctx.Request().URL.Path
	})

	s.handlers = map[string]http.Handler{
		http.MethodGet:    m.Handler(s.getState),
		http.MethodPost:   m.Handler(s.updateState),
		http.MethodDelete: m.Handler(s.deleteState),
		"LOCK":            m.Handler(s.lockState),
		"UNLOCK":          m.Handler(s.unlockState),
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, ok := s.handlers[r.Method]
	if ok {
		h.ServeHTTP(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getState(c *Context) {
	bytes, err := s.bucket.GetBytes(c.name)
	if err != nil {
		c.Error(err)
		return
	}

	c.ResponseWriter().Write(bytes)
}

func (s *Server) updateState(c *Context) {
	bytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		c.Error(err)
		return
	}

	_, err = s.bucket.PutBytes(c.name, bytes)
	if err != nil {
		c.Error(err)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}

func (s *Server) deleteState(c *Context) {
	err := s.bucket.Delete(c.name)
	if err != nil {
		c.Error(err)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}

func (s *Server) lockState(c *Context) {
	bytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		c.Error(err)
		return
	}

	log.Printf("Locking %s:%s", c.name, string(bytes))

	_, err = s.locks.Create(c.name, bytes)
	if err != nil {
		http.Error(c.ResponseWriter(), "Locked", http.StatusLocked)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}

func (s *Server) unlockState(c *Context) {
	log.Printf("Unlocking %s", c.name)

	err := s.locks.Delete(c.name)
	if err != nil {
		c.Error(err)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}
