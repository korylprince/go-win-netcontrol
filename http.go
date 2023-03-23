package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hectane/go-acl"
	"github.com/rs/zerolog"
)

var sockPath = filepath.Join(ServiceConfig.InstallPath, "control.sock")

var errUnauthorized = errors.New("unauthorized")

type request struct {
	Password string `json:"string"`
	Enabled  bool   `json:"enabled"`
}

type response struct {
	Error string `json:"error"`
}

// Server runs in an elevated Windows service to make network inferface changes
type Server struct {
	Logger   zerolog.Logger
	listener net.Listener
	server   *http.Server
}

// NewServer returns a new Server with the given logger
func NewServer(logger zerolog.Logger) (*Server, error) {
	if err := os.RemoveAll(sockPath); err != nil {
		return nil, fmt.Errorf("could not remove socket: %w", err)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("could not listen on socket: %w", err)
	}

	if err := acl.Chmod(sockPath, 0666); err != nil {
		return nil, fmt.Errorf("could not set socket permissions: %w", err)
	}

	s := &Server{Logger: logger, listener: listener}

	return s, nil
}

// Serve serves HTTP on a unix socket until an error occurs
func (s *Server) Serve() error {
	server := &http.Server{Handler: http.HandlerFunc(s.SetStatus)}
	s.server = server
	return server.Serve(s.listener)
}

// Shutdown shuts down the server
func (s *Server) Shutdown() error {
	if err := s.server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("could not shutdown server: %w", err)
	}

	return nil
}

// SetStatus is an HTTP handler that verifies a password and enables or disables network interfaces
func (s *Server) SetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		s.Logger.Warn().Err(fmt.Errorf("invalid method: %s", r.Method)).Send()
		return
	}

	req := new(request)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Warn().Err(fmt.Errorf("could not decode request: %w", err)).Send()
		return
	}

	if !Validate(req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		s.Logger.Warn().Msg("invalid password")
		return
	}

	// recover panics, because WMI seems to be pretty buggy
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("panic: %v", r)
			s.Logger.Error().Msg(errStr)
			resp := &response{Error: "Error (panic): Please try again later"}
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				s.Logger.Error().Err(fmt.Errorf("could not encode response: %w", err)).Send()
			}
		}
	}()

	conn, err := NewConn()
	if err != nil {
		err = fmt.Errorf("could not create WMI conn: %w", err)
		s.Logger.Error().Err(err).Send()
		resp := &response{Error: "Error (WMI conn): Please try again later"}
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.Logger.Error().Err(fmt.Errorf("could not encode response: %w", err)).Send()
		}
		return
	}
	defer conn.Close()

	if err = conn.SetStatus(req.Enabled); err != nil {
		err = fmt.Errorf("could not set status: %w", err)
		s.Logger.Error().Err(err).Send()
		resp := &response{Error: "Error (SetStatus): Please try again later"}
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.Logger.Error().Err(fmt.Errorf("could not encode response: %w", err)).Send()
		}
		return
	}

	method := "Disabled"
	if req.Enabled {
		method = "Enabled"
	}
	w.WriteHeader(http.StatusOK)
	s.Logger.Info().Msg(fmt.Sprintf("interfaces set to %s", method))
}

// Client is a client for Server
type Client struct {
	client *http.Client
}

// NewClient returns a new Client
func NewClient() *Client {
	return &Client{client: &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}}
}

// SetStatus requests to change network interface statuses
func (c *Client) SetStatus(passwd string, enabled bool) error {
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(&request{Password: passwd, Enabled: enabled}); err != nil {
		return fmt.Errorf("could not encode body: %w", err)
	}

	resp, err := c.client.Post("http://unix/", "application/json", body)
	if err != nil {
		return fmt.Errorf("could not post request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return errUnauthorized
	case http.StatusInternalServerError:
		r := new(response)
		if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
			return fmt.Errorf("could not decode response: %w", err)
		}
		return errors.New(r.Error)
	default:
		return fmt.Errorf("unexpected status: %d %s", resp.StatusCode, resp.Status)
	}
}
