package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/service"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// loggingListener wraps a net.Listener and logs every accepted connection.
type loggingListener struct {
	net.Listener
}

func (l loggingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		log.Printf("[gRPC] Accept error: %v", err)
		return nil, err
	}
	log.Printf("[gRPC] New connection from %v", conn.RemoteAddr())
	return conn, nil
}

func main() {
	grpcPort := flag.Int("grpc-port", 7777, "gRPC server port")
	httpPort := flag.Int("http-port", 8080, "HTTP server port (Octo API)")
	host := flag.String("host", "127.0.0.1", "hostname the client will connect to")
	resourcesBaseURL := flag.String("resources-base-url", "", "Resources base URL for list.bin rewrite (must be exactly 43 chars); empty = derive from host (so client uses our server for assets)")
	flag.Parse()

	// Octo base URL: client uses this to fetch list and will see rewritten asset URLs if resourcesBaseURL is set
	octoURL := "http://" + *host + ":" + strconv.Itoa(*httpPort)
	if *resourcesBaseURL == "" {
		// Default: rewrite list.bin so asset base URL points to our server (must be exactly 43 chars for protobuf).
		// Use http:// so the client uses plain HTTP; our server does not speak TLS on this port.
		candidate := "http://" + *host + ":" + strconv.Itoa(*httpPort) + "/resource-bundle-server"
		if len(candidate) == 43 {
			*resourcesBaseURL = candidate
		}
	}
	if *resourcesBaseURL != "" && len(*resourcesBaseURL) != 43 {
		log.Printf("[config] resources-base-url length is %d (need 43); list.bin will be served unchanged", len(*resourcesBaseURL))
		*resourcesBaseURL = ""
	}

	// Start HTTP server for Octo API and general game HTTP (HTTP/1.1 + HTTP/2 cleartext)
	octoServer := service.NewOctoHTTPServer(*resourcesBaseURL)
	h2s := &http2.Server{}
	octoHandler := h2c.NewHandler(octoServer.Handler(), h2s)
	go func() {
		log.Printf("Octo HTTP server listening on :%d (HTTP/1.1 + h2c)", *httpPort)
		srv := &http.Server{Addr: fmt.Sprintf(":%d", *httpPort), Handler: octoHandler}
		http2.ConfigureServer(srv, h2s)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("HTTP server on %d failed: %v", *httpPort, err)
		}
	}()
	// Also listen on port 80 for plain HTTP requests (game web API)
	go func() {
		log.Printf("HTTP server also listening on :80 (HTTP/1.1 + h2c)")
		srv80 := &http.Server{Addr: ":80", Handler: octoHandler}
		http2.ConfigureServer(srv80, h2s)
		if err := srv80.ListenAndServe(); err != nil {
			log.Printf("HTTP server on :80 failed (non-fatal): %v", err)
		}
	}()
	// Start gRPC server (plaintext — client TLS is disabled via Frida / APK patches)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	lis = loggingListener{Listener: lis}

	// Also listen gRPC on 443 when client uses host-only patch (default port 443)
	lis443, err443 := net.Listen("tcp", ":443")
	if err443 != nil {
		log.Printf("gRPC on :443 skipped (need sudo or port in use): %v", err443)
	} else {
		lis443 = loggingListener{Listener: lis443}
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	pb.RegisterUserServiceServer(grpcServer, service.NewUserServiceServer())
	pb.RegisterConfigServiceServer(grpcServer, service.NewConfigServiceServer(*host, int32(*grpcPort), octoURL))
	pb.RegisterDataServiceServer(grpcServer, service.NewDataServiceServer())
	pb.RegisterTutorialServiceServer(grpcServer, service.NewTutorialServiceServer())
	pb.RegisterGamePlayServiceServer(grpcServer, service.NewGameplayServiceServer())
	pb.RegisterQuestServiceServer(grpcServer, service.NewQuestServiceServer())
	pb.RegisterNotificationServiceServer(grpcServer, service.NewNotificationServiceServer())

	reflection.Register(grpcServer)

	log.Printf("gRPC server listening on :%d", *grpcPort)
	if lis443 != nil {
		go grpcServer.Serve(lis443)
		log.Printf("gRPC server also listening on :443 (for host-only patched client)")
	}
	log.Printf("client host address: %s:%d", *host, *grpcPort)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	log.Printf(">>> %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		log.Printf("<<< %s ERROR: %v", info.FullMethod, err)
	} else {
		log.Printf("<<< %s OK", info.FullMethod)
	}
	return resp, err
}
