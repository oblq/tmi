## build		: (usage: sudo make build path=/opt/ipmifc) build the go program for the current environment, copy the executable and the config files to the specified path.
.PHONY: build
build:
	@\
	/usr/local/go/bin/go build -i -ldflags "-X main.Path=$(path)" -o $(path)/ipmifc . ; \
    cp -n ./artifacts/ipmifc.yaml $(path)/ipmifc.yaml && chmod 777 $(path)/ipmifc.yaml; \
    cp -n ./artifacts/ipmifc_thresholds.yaml $(path)/ipmifc_thresholds.yaml && chmod 777 $(path)/ipmifc_thresholds.yaml;

# !!run with sudo
## install_linux	: (usage: sudo make install_linux path=/opt/ipmifc) run build, copy_files, install `ipmitool` and create a systemctl service to run the application as a daemon.
.PHONY: install_linux
install_linux: build
	@\
	apt-get -qy install ipmitool; \
	sed -e "s@<path>@$(path)/ipmifc@" ./artifacts/ipmifc.service > /etc/systemd/system/ipmifc.service;
	systemctl daemon-reload; \
	systemctl enable ipmifc.service; \
	systemctl stop ipmifc.service; \
	systemctl start ipmifc.service; \
	watch systemctl status ipmifc.service;

## logs_linux	: (usage: make logs_linux) logs the application stdout.
.PHONY: logs_linux
logs_linux:
	@ journalctl -f -u ipmifc;

## build_multiplatform	: (usage: sudo make build_multiplatform) build executable for win, mac and linux.
.PHONY: build_multiplatform
build_multiplatform:
	@ ./artifacts/bin/build.sh ../ipmifc ./artifacts/bin;

.PHONY : help
help : Makefile
	@sed -n 's/^##//p' $<
