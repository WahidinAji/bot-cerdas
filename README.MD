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

# kenapa bot ini ada?
berawal dari perbincangan saya dan kawan-kawan saya di discord. Kami sering membalas opini bodoh dengan cerdas, akan tetapi salah satu dari kami bilang `capek juga ngetik cerdas terus`. Nah, maka dari itu saya membuat bot ini.

# kenapa saya ngerti cerdas?
saya ngerti cerdas karena saya punya cerdas, dan saya punya cerdas. Kalau saya punya cerdas, saya punya cerdas.

https://discord.com/channels/<guild_id_or_server>/<channel_id>