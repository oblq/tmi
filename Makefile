## build		: (usage: sudo make build path=/opt/tmi) build the go program for the current environment, copy the executable and the config files to the specified path.
.PHONY: build
build:
	/usr/local/go/bin/go build -i -ldflags "-X main.Path=$(path)" -o $(path)/tmi . ; \
    cp -n ./artifacts/tmi.yaml $(path)/tmi.yaml && chmod 666 $(path)/tmi.yaml; \
    cp -n ./artifacts/ipmi.yaml $(path)/ipmi.yaml && chmod 666 $(path)/ipmi.yaml; \
    cp -n ./artifacts/commanderpro.yaml $(path)/commanderpro.yaml && chmod 666 $(path)/commanderpro.yaml;

# !!run with sudo
## install_linux	: (usage: sudo make install_linux path=/opt/tmi) run build, copy_files, install `ipmitool` and create a systemctl service to run the application as a daemon.
.PHONY: install_linux
install_linux: build
	apt-get -qy install ipmitool; \
	apt-get -qy install libusb-1.0-0-dev; \
	sed -e "s@<path>@$(path)/tmi@" ./artifacts/tmi.service > /etc/systemd/system/tmi.service;
	systemctl daemon-reload; \
	systemctl enable tmi.service; \
	systemctl stop tmi.service; \
	systemctl start tmi.service; \
	watch systemctl status tmi.service;

## logs_linux	: (usage: make logs_linux) logs the application stdout.
.PHONY: logs_linux
logs_linux:
	@ journalctl -f -u tmi;

## build_multiplatform	: (usage: sudo make build_multiplatform) build executable for win, mac and linux.
.PHONY: build_multiplatform
build_multiplatform:
	@ ./artifacts/bin/build.sh ../tmi ./artifacts/bin;

.PHONY : help
help : Makefile
	@sed -n 's/^##//p' $<
