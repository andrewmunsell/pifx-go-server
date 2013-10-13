ssh pi@raspberry-pi sudo killall pifx-go-server

scp pifx-go-server pi@raspberry-pi:/home/pi/pifx

sleep 0.5

ssh pi@raspberry-pi nohup sudo /home/pi/pifx/pifx-go-server -tcp > /dev/null 2>&1&