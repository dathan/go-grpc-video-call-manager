# Always give credit @campoy
# https://github.com/campoy/justforfunc/blob/master/12-say-grpc/api/Makefile
#
build:
	protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	session.proto
