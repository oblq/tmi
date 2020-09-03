## build_plugins		: (usage: make build_plugins) build the go plugins for the current environment, the *.so files and their configs must be copied to the final TMI path.
.PHONY: build_plugins
build_plugins:
	@ ./plugins/build.sh

## build		: (usage: sudo make build path=/opt/tmi) build the go program for the current environment, copy the executable and the config files to the specified path.
.PHONY: build
build:
	go build -trimpath -ldflags "-X main.Path=$(path)" -o $(path)/tmi . ; \
    cp -n ./tmi.yaml $(path)/tmi.yaml; # && chmod 666 $(path)/tmi.yaml;
#    cp -n ./artifacts/ipmi.yaml $(path)/ipmi.yaml && chmod 666 $(path)/ipmi.yaml; \
#    cp -n ./artifacts/commanderpro.yaml $(path)/commanderpro.yaml && chmod 666 $(path)/commanderpro.yaml;

# !!run with sudo, -E or --preserve-env=PATH
## install_linux	: (usage: sudo --preserve-env=PATH make install_linux path=/opt/tmi) run build, copy_files, install `ipmitool` and create a systemctl service to run the application as a daemon.
.PHONY: install_linux
install_linux: build_plugins build
	apt-get -qy install ipmitool; \
	apt-get -qy install libusb-1.0-0-dev; \
	sed -e "s@<path>@$(path)/tmi@" ./tmi.service > /etc/systemd/system/tmi.service;
	systemctl daemon-reload; \
	systemctl enable tmi.service; \
	systemctl stop tmi.service; \
	systemctl start tmi.service; \
	watch systemctl status tmi.service;

## logs_linux	: (usage: make logs_linux) logs the application stdout.
.PHONY: logs_linux
logs_linux:
	@ journalctl -f -u tmi;

## build_multiplatform	: (usage: sudo make build_multiplatform) build executable for win, mac and linux. The executable will look for the configs in its working dir by default.
.PHONY: build_multiplatform
build_multiplatform:
	@ ./build.sh ../tmi ./;

.PHONY : help
help : Makefile
	@sed -n 's/^##//p' $<
