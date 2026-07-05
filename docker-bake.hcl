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
    "ghcr.io/thegeb/blt-volume-manager-plugin:${VERSION}",
    "ghcr.io/thegeb/blt-volume-manager-plugin:latest"
  ]
  args = {
    VERSION = VERSION
    COMMIT = COMMIT
  }
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
  provenance = false
  sbom = false
}

target "web" {
  target = "web"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = [
    "ghcr.io/thegeb/blt-volume-manager-web:${VERSION}",
    "ghcr.io/thegeb/blt-volume-manager-web:latest"
  ]
  args = {
    VERSION = VERSION
    COMMIT = COMMIT
  }
  cache-from = ["type=gha"]
  cache-to = ["type=gha,mode=max"]
  provenance = false
  sbom = false
}
