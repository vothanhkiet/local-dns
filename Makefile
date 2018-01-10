VERSION=1.0.0
BUILD=`git rev-list master --first-parent --count`
DATE=`date +%FT%T%z`
FLAGS=-ldflags "-extldflags '-lm -lstdc++ -static' -X main.version=${VERSION} -X main.build=${BUILD} -X main.date=${DATE}"

window:
	GOOS=windows GOARCH=amd64 go build ${FLAGS}

linux:
	GOOS=linux GOARCH=amd64 go build ${FLAGS}

mac:
	GOOS=darwin GOARCH=amd64 go build ${FLAGS}

clean:
	rm -rf logs
	rm -rf debug
	rm -rf local-dns
