package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

const (
	CTRLPORT       string = "47921"
	TCPPROXYBASE   int    = 47923
	TCPPROXYAMOUNT int    = 10
)

type Server struct {
	proxy *Proxy
}

func (s *Server) run(context context.Context) {
	defer wg.Done()
	config := prepareTlsConfig()
	if config == nil {
		logger.Error("Error preparing TLS config:", nil)
		return
	}

	for {
		select {
		case <-context.Done():
			return
		default:
			logger.Info("Waiting for client to connect", slog.String("Port", CTRLPORT))
			s.waitForCtrlConnection(context, config)
			logger.Info("Client connected", slog.String("IP", s.proxy.CtrlConn.RemoteAddr().String()))
			// Run a goroutine that will handle all writes to the ctrl connection
			wg.Add(1)
			go s.proxy.manageCtrlConnectionOutgoing(context)
			// Keep reading from the ctrl connection till disconnected or closed
			s.proxy.manageCtrlConnectionIncoming(context)
			logger.Info("Client disconnected", slog.String("IP", s.proxy.CtrlConn.RemoteAddr().String()))
			// clean up
		}
	}
}

func prepareTlsConfig() *tls.Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Error getting home directory:", err)
		return nil
	}
	filePath := filepath.Join(homeDir, "certs", "myCA.pem")
	caCertData, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("Error reading CA certificate:", err)
		return nil
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCertData)
	if !ok {
		logger.Error("Error appending CA certificate to pool.", nil)
		return nil
	}
	keyPath := filepath.Join(homeDir, "certs", "server.key")
	crtPath := filepath.Join(homeDir, "certs", "server.crt")
	cer, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		logger.Error("Error loading key pair:", err)
		return nil
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cer},
		ClientCAs:    caCertPool,
		// The main purpose of this is to verify the client certificate
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	return tlsConfig
}

func (s *Server) waitForCtrlConnection(ctx context.Context, config *tls.Config) {
	l, err := tls.Listen("tcp", ":"+CTRLPORT, config)
	if err != nil {
		logger.Error("Error TLS listening", slog.String("Port", CTRLPORT), "Error", err)
		panic(err)
	}
	listeningCtx, listCancel := context.WithCancel(ctx)
	defer listCancel()

	// Run a helper goroutine to close the listener when stop is received from console
	wg.Add(1)
	go func(ctx context.Context, l net.Listener) {
		defer wg.Done()
		logger.Debug("Starting TLS listener")
		<-ctx.Done()
		logger.Debug("Closing TLS listener")
		err := l.Close()
		if err != nil {
			logger.Debug("Error closing TLS listener:", err)
		}
		l = nil
		logger.Debug("Stopping TLS listener")
	}(listeningCtx, l)

	conn, err := l.Accept()
	if err != nil {
		logger.Debug("Error accepting connection:", err)
		return
	}

	logger.Debug("Accepted connection, starting proxy", slog.String("Address", conn.RemoteAddr().String()))

	s.proxy = NewProxy(conn)
	return
}
