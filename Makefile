all: yadisk-ds-sync-linux yadisk-ds-sync

yadisk-ds-sync:
	cd src && go build -o ../yadisk-ds-sync .

yadisk-ds-sync-linux:
	cd src && GOOS=linux GOARCH=amd64 go build -o ../yadisk-ds-sync-linux .