package http_no

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// StopOnSignal runs a background process that listens for operating system
// signals and shuts down the HTTP server when a signal is received. If more
// than one HTTP service is running, can accept multiple servers to stop them
// all when signals are tripped.
func StopOnSignal(servers ...*Server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func(s []*Server) {
		<-signals

		for _, server := range s {
			server.Stop()
		}
	}(servers)
}

// WaitForSignal calls StopOnSignal and waits.
func WaitForSignal(servers ...*Server) {
	StopOnSignal(servers...)
	for _, s := range servers {
		s.Wait()
	}
}

// ServerOption is a configuration option used when constructing a Server
type ServerOption func(s *Server)

// IdleTimeout sets the server's IdleTimeout.
func IdleTimeout(t time.Duration) ServerOption {
	return func(s *Server) {
		s.Server.IdleTimeout = t
	}
}

// ReadTimeout sets the server's ReadTimeout.
func ReadTimeout(t time.Duration) ServerOption {
	return func(s *Server) {
		s.Server.ReadTimeout = t
	}
}

// TLS configures the server certs.
func TLS(certContents, keyContents []byte) ServerOption {
	return func(s *Server) {
		cert, err := tls.X509KeyPair(certContents, keyContents)
		if err != nil {
			//Logger.Printf("Error generating X509KeyPair: %s", err)
			return
		}
		s.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}
}

// WriteTimeout sets the server's WriteTimeout.
func WriteTimeout(t time.Duration) ServerOption {
	return func(s *Server) {
		s.Server.WriteTimeout = t
	}
}

// NewServer constructs a new web server zero or more server options.
// The resulting server supports graceful shutdown. i.e. the server will wait
// until existing all connections complete or time out before shutting down
func NewServer(listen string, handler http.Handler, options ...ServerOption) *Server {
	s := &Server{
		Server: &http.Server{
			Addr:           listen,
			Handler:        handler,
			MaxHeaderBytes: 1 << 20,
		},
		close: make(chan bool),
		done:  make(chan bool),
	}
	for i := range options {
		options[i](s)
	}
	return s
}

// Server supports graceful exits and multiple servers per application (i.e.
// listen to multiple ports).
type Server struct {
	*http.Server
	// tls determines whether to Serve TLS
	tls bool
	// close triggers a graceful shutdown of the server.
	close chan bool
	// done indicates the server has completed shutting down.
	done chan bool
}

// Close the server. Will try to gracefully shutdown, but if the server takes
// longer than 5 seconds to stop, forcibly shuts it down.
func (s *Server) Close() error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return s.Shutdown(ctx)
}

// Start an HTTP server in a separate process and wait for a close signal to
// shutdown.
//
// When a "close" message is received, the server will shutdown gracefully (i.e.
// wait for any unfinished requests to complete).  It will send a message to the
// "done" channel when the server has stopped.
func (s *Server) Start() {
	// Wait for a "close" message in the background to stop the server.
	go func() {
		<-s.close
		s.Close()
	}()

	go func() {
		if s.TLSConfig != nil {
			if err := s.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				//Logger.Printf("Error starting server: %s", err)
				s.Stop()
			}
		} else {
			if err := s.ListenAndServe(); err != http.ErrServerClosed {
				//Logger.Printf("Error starting server: %s", err)
				s.Stop()
			}
		}
		s.done <- true
	}()

	// Give the server some time to start...
	time.Sleep(10 * time.Millisecond)
}

// Stop the HTTP server gracefully. This function sends a message to the server
// to stop, and will return immediately. Call Wait() to wait for the server to
// shutdown.
func (s *Server) Stop() {
	s.close <- true
}

// Wait for the server to shutdown. This call will block, so do any prep
// work before calling this function.
func (s *Server) Wait() {
	<-s.done
}

// Protocol returns the protocol supported by this server (http or https).
func (s *Server) Protocol() string {
	if s.Server.TLSConfig != nil {
		return "https"
	}
	return "http"
}
