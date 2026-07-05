variable "VERSION" {
  default = "dev"
}

variable "COMMIT" {
  default = "unknown"
}

variable "REGISTRY" {
  default = "ghcr.io/thegeb/blt-volume-manager"
}

group "default" {
  targets = ["plugin", "web"]
}

target "plugin" {
  target = "plugin"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = [
    "${REGISTRY}:${VERSION}",
    "${REGISTRY}:latest"
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
    "${REGISTRY}:${VERSION}-web",
    "${REGISTRY}:latest-web"
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
