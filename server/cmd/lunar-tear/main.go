package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	grpcPort := flag.Int("grpc-port", 7777, "gRPC server port")
	httpPort := flag.Int("http-port", 8080, "HTTP server port (Octo API)")
	host := flag.String("host", "10.0.2.2", "hostname the client will connect to (10.0.2.2 = host from Android emulator)")
	flag.Parse()

	// Start HTTP server for Octo API and general game HTTP
	octoServer := service.NewOctoHTTPServer()
	go func() {
		log.Printf("Octo HTTP server listening on :%d", *httpPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), octoServer.Handler()); err != nil {
			log.Fatalf("HTTP server on %d failed: %v", *httpPort, err)
		}
	}()
	// Also listen on port 80 for plain HTTP requests (game web API)
	go func() {
		log.Printf("HTTP server also listening on :80")
		if err := http.ListenAndServe(":80", octoServer.Handler()); err != nil {
			log.Printf("HTTP server on :80 failed (non-fatal): %v", err)
		}
	}()
	// Listen on port 443 with TLS for HTTPS requests (game web API)
	go func() {
		log.Printf("HTTPS server listening on :443")
		if err := http.ListenAndServeTLS(":443", "certs/cert.pem", "certs/key.pem", octoServer.Handler()); err != nil {
			log.Printf("HTTPS server on :443 failed (non-fatal): %v", err)
		}
	}()

	// Start gRPC server (plaintext — client TLS is disabled via Frida)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	pb.RegisterUserServiceServer(grpcServer, service.NewUserServiceServer())
	pb.RegisterConfigServiceServer(grpcServer, service.NewConfigServiceServer(*host, int32(*grpcPort)))
	pb.RegisterDataServiceServer(grpcServer, service.NewDataServiceServer())
	pb.RegisterTutorialServiceServer(grpcServer, service.NewTutorialServiceServer())
	pb.RegisterGamePlayServiceServer(grpcServer, service.NewGamePlayServiceServer())
	pb.RegisterQuestServiceServer(grpcServer, service.NewQuestServiceServer())
	pb.RegisterNotificationServiceServer(grpcServer, service.NewNotificationServiceServer())

	// Also register services under the client's expected full package paths
	dataAltDesc := pb.DataService_ServiceDesc
	dataAltDesc.ServiceName = "apb.api.data.DataService"
	grpcServer.RegisterService(&dataAltDesc, service.NewDataServiceServer())

	gameplayAltDesc := pb.GamePlayService_ServiceDesc
	gameplayAltDesc.ServiceName = "apb.api.gameplay.GamePlayService"
	grpcServer.RegisterService(&gameplayAltDesc, service.NewGamePlayServiceServer())

	questAltDesc := pb.QuestService_ServiceDesc
	questAltDesc.ServiceName = "apb.api.quest.QuestService"
	grpcServer.RegisterService(&questAltDesc, service.NewQuestServiceServer())

	notifAltDesc := pb.NotificationService_ServiceDesc
	notifAltDesc.ServiceName = "apb.api.notification.NotificationService"
	grpcServer.RegisterService(&notifAltDesc, service.NewNotificationServiceServer())

	reflection.Register(grpcServer)

	log.Printf("gRPC server listening on :%d", *grpcPort)
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
