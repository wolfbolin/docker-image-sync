package hub

import (
	"strings"

	"github.com/asaskevich/govalidator/v12"
)

type Image struct {
	Registry string
	Project  string
	Name     string
}

func ParseImage(image string) *Image {
	parts := strings.SplitN(image, "/", 3)
	switch len(parts) {
	case 1:
		return &Image{Name: parts[0]}
	case 2:
		if isRegistry(parts[0]) {
			return &Image{Registry: parts[0], Name: parts[1]}
		}
		return &Image{Project: parts[0], Name: parts[1]}
	default:
		if isRegistry(parts[0]) {
			return &Image{Registry: parts[0], Project: parts[1], Name: parts[2]}
		}
		return &Image{Project: parts[0], Name: parts[1] + "/" + parts[2]}
	}
}

func (r *Image) ToUrl() string {
	var parts []string
	if r.Registry != "" {
		parts = append(parts, r.Registry)
	}
	if r.Project != "" {
		parts = append(parts, r.Project)
	}
	parts = append(parts, r.Name)
	return strings.Join(parts, "/")
}

func (r *Image) ToTagUrl(tag string) string {
	return r.ToUrl() + ":" + tag
}

func isRegistry(part string) bool {
	if govalidator.IsHost(part) {
		return true
	} else if parts := strings.Split(part, ":"); len(parts) == 2 {
		if govalidator.IsHost(parts[0]) && govalidator.IsPort(parts[1]) {
			return true
		}
	}
	return false
}

func IsSchemaV1(mediaType string) bool {
	return strings.HasPrefix(mediaType, "application/vnd.docker.distribution.manifest.v1")
}
