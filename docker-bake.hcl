variable "VERSION" {
  default = "dev"
}

variable "COMMIT" {
  default = "unknown"
}

group "default" {
  targets = ["plugin", "web"]
}

target "plugin" {
  target = "plugin"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = [
    "ghcr.io/thegeb/blt-volume-manager-plugin:v${trimprefix(VERSION, "v")}",
    "ghcr.io/thegeb/blt-volume-manager-plugin:${trimprefix(VERSION, "v")}",
    "ghcr.io/thegeb/blt-volume-manager-plugin:latest"
  ]
  args = {
    VERSION = VERSION
    COMMIT = COMMIT
  }
  cache-from = ["type=gha,scope=plugin"]
  cache-to = ["type=gha,mode=max,scope=plugin"]
}

target "web" {
  target = "web"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = [
    "ghcr.io/thegeb/blt-volume-manager-web:v${trimprefix(VERSION, "v")}",
    "ghcr.io/thegeb/blt-volume-manager-web:${trimprefix(VERSION, "v")}",
    "ghcr.io/thegeb/blt-volume-manager-web:latest"
  ]
  args = {
    VERSION = VERSION
    COMMIT = COMMIT
  }
  cache-from = ["type=gha,scope=web"]
  cache-to = ["type=gha,mode=max,scope=web"]
}
