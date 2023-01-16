package main

import (
	"io"
	"log"
	"net/http"
	"strings"

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

	m.Before(func(c *Context) {
		c.name = strings.TrimPrefix(c.Request().URL.Path, "/")
	})

	m.Before(func(c *Context) {
		log.Printf("%s %s\n", c.Request().Method, c.name)
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

	_, err = s.locks.Create(c.name, bytes)
	if err != nil {
		http.Error(c.ResponseWriter(), "Locked", http.StatusLocked)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}

func (s *Server) unlockState(c *Context) {
	err := s.locks.Delete(c.name)
	if err != nil {
		c.Error(err)
		return
	}

	c.ResponseWriter().WriteHeader(http.StatusOK)
}
