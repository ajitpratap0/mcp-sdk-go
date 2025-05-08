package protocol

import (
	"encoding/json"
	"time"
)

// Resource represents a resource in the MCP protocol
type Resource struct {
	URI         string            `json:"uri"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"`
	Size        int64             `json:"size,omitempty"`
	ModTime     time.Time         `json:"modTime,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Children    []string          `json:"children,omitempty"`
	TemplateURI string            `json:"templateUri,omitempty"`
}

// ResourceTemplate defines a template for dynamically generated resources
type ResourceTemplate struct {
	URI          string                 `json:"uri"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Type         string                 `json:"type"`
	Parameters   []ResourceParameter    `json:"parameters"`
	ParameterDefs map[string]interface{} `json:"parameterDefs,omitempty"`
	Examples     []ResourceExample      `json:"examples,omitempty"`
}

// ResourceParameter defines a parameter for a resource template
type ResourceParameter struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"`
}

// ResourceExample provides an example of resource template parameters
type ResourceExample struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	URI        string                 `json:"uri,omitempty"`
}

// ResourceContents contains the content of a resource
type ResourceContents struct {
	URI      string          `json:"uri"`
	Type     string          `json:"type"`
	Content  json.RawMessage `json:"content"`
	Encoding string          `json:"encoding,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ListResourcesParams defines parameters for listing resources
type ListResourcesParams struct {
	URI      string `json:"uri,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
	PaginationParams
}

// ListResourcesResult defines the response for listing resources
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
	Templates []ResourceTemplate `json:"templates,omitempty"`
	PaginationResult
}

// ReadResourceParams defines parameters for reading a resource
type ReadResourceParams struct {
	URI            string                 `json:"uri"`
	TemplateParams map[string]interface{} `json:"templateParams,omitempty"`
	Range          *ResourceRange         `json:"range,omitempty"`
}

// ResourceRange specifies a range within a resource
type ResourceRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end,omitempty"`
}

// ReadResourceResult defines the response for reading a resource
type ReadResourceResult struct {
	Contents ResourceContents `json:"contents"`
}

// SubscribeResourceParams defines parameters for subscribing to resource changes
type SubscribeResourceParams struct {
	URI       string `json:"uri"`
	Recursive bool   `json:"recursive,omitempty"`
}

// SubscribeResourceResult defines the response for resource subscription
type SubscribeResourceResult struct {
	Success bool `json:"success"`
}

// ResourcesChangedParams defines parameters for the resourcesChanged notification
type ResourcesChangedParams struct {
	URI       string     `json:"uri"`
	Resources []Resource `json:"resources,omitempty"`
	Removed   []string   `json:"removed,omitempty"`
	Added     []Resource `json:"added,omitempty"`
	Modified  []Resource `json:"modified,omitempty"`
}

// ResourceUpdatedParams defines parameters for the resourceUpdated notification
type ResourceUpdatedParams struct {
	URI      string          `json:"uri"`
	Contents ResourceContents `json:"contents,omitempty"`
	Deleted  bool            `json:"deleted,omitempty"`
}
