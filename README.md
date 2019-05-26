# pornhub-dl

pornhub-dl started as a fun project to learn Go and to demonstrate a few basic capabilities of the Go language. The whole project is inspired by [youtube-dl](https://github.com/ytdl-org/youtube-dl/). It was done in about 2 hours with absolutely zero knowledge of Go. 

## Usage
Clone this repository and build it by yourself with `go build` or download one of the prebuilt assemblies.
You can call it via command-line. It supports the following flags:

|Flag|Default|Description|
|----|-------|-----------|
|url|"empty"|URL of the video to download|
|quality|"highest"|The quality number (eg. 720) or 'highest'|
|output|"default"|Path to where the download should be saved or 'default' for the original filename|
|threads|10|The amount of simultaneous download streams|
|socket5|""|Specify socks5 proxy address for downloading resources|
|debug|false|Whether you want to activate debug mode or not (not in use)|

## Contribution
Contributions of any kind such as refactorings with explanations or adding new features are appreciated.

## License
Please see the [LICENSE](https://github.com/festie/pornhub-dl/blob/master/LICENSE) file for more information.