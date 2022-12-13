# SSD Test

![ssd-test](https://user-images.githubusercontent.com/20717116/207364177-61609f68-d09b-49c9-9946-a2eac00c643c.png)

This is a simple test to check the write speed of your SSD.

Because SSD have a small amount of RAM to cache the data,
if you just write a few GB, the speed will be very high.
But if you continuously write a lot of data, after the cache is full,
the speed will drop to the real-speed of the SSD.

This tool will create a file and write it continuously,
until the file takes up all the space of the SSD.
During the test, the speed will be shown in the terminal.

Then it will delete the file.

## Usage

```bash
go run github.com/openhoangnc/ssd-test@latest
```

