# SSD Test

![ssd-test](https://user-images.githubusercontent.com/20717116/207495720-ffb9c971-edf7-4f8a-97bb-e3a87c7e514b.png)

A simple tool to check and analyze the real write speed of your SSD.

## Overview

SSD drives typically have a small amount of RAM to cache data, resulting in very high initial write speeds when writing just a few GB. However, once this cache is filled during continuous writing, the speed drops to the actual performance level of the SSD.

This tool helps you measure this real-world performance by:
- Creating a test file that continuously writes data
- Filling available disk space to test beyond the cache limit
- Displaying real-time write speed in the terminal
- Automatically deleting the test file when complete

## Installation

```bash
go install github.com/openhoangnc/ssd-test@latest
```

## Usage

Run directly without installation:

```bash
go run github.com/openhoangnc/ssd-test@latest
```

Or if installed:

```bash
ssd-test
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
