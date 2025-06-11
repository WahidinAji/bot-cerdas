# jadi gini

iya gitu

# saya deploynya di lokal server sih, dan gini kurang lebih

## copy dulu pake scp dari macbook gue ke komputer server lokal gue
```sh
scp main.go duniya@192.168.1.8:/home/duniya/cerdas/
```
## sila anu jadi
```sh
sudo rm cerdas
go build -o cerdas main.go
chmod +x cerdas
sudo cp cerdas /opt/cerdas/cerdas
sudo systemctl daemon-reload
sudo systemctl start cerdas
sudo systemctl status cerdas
```

## systemd service
ya kalian cari sendiri lah gimana buatnya, banyak di internet. Kalau malas ya pakai ei ay aja bro


# jangan lupa environment
```sh
export DISCORD_BOT_TOKEN=XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

kalau gatau apa itu environment, kerja keras lagi ya.
