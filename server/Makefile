# Proto generation: outputs to gen/proto/ (module=lunar-tear/server).
# All proto files have go_package. Only protos used by the server are generated here
# (generating all would put them in one package and cause name clashes).
PROTO_USED = proto/config.proto proto/data.proto proto/deck.proto proto/gacha.proto proto/gameplay.proto proto/gimmick.proto proto/mission.proto proto/notification.proto proto/quest.proto proto/tutorial.proto proto/user.proto

proto:
	protoc -I . $(PROTO_USED) --go_out=. --go_opt=module=lunar-tear/server --go-grpc_out=. --go-grpc_opt=module=lunar-tear/server
	@echo "Generated in gen/proto/"

.PHONY: proto
