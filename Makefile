plugin:
	-@kill-streamdeck.bat
	@go build -o com.exension.stocks.sdPlugin\\sdplugin-stocks.exe github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
	@env GOOS=darwin GOARCH=amd64 go build -o com.exension.stocks.sdPlugin\\sdplugin-stocks github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
	@xcopy com.exension.stocks.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.exension.stocks.sdPlugin\\ /E /Q /Y
	@start-streamdeck.bat

debug:
	@go build -o com.exension.stocks.sdPlugin\\sdplugin-stocks.exe github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
	@env GOOS=darwin GOARCH=amd64 go build -o com.exension.stocks.sdPlugin\\sdplugin-stocks github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
	@xcopy com.exension.stocks.sdPlugin $(APPDATA)\\Elgato\\StreamDeck\\Plugins\\com.exension.stocks.sdPlugin\\ /E /Q /Y

release:
	-@rm release\com.exension.stocks.streamDeckPlugin
	@DistributionTool.exe -b -i com.exension.stocks.sdPlugin -o release

.PHONY: release