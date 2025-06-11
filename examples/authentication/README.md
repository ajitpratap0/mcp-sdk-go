# MCP Authentication Example

This example demonstrates how to use the authentication framework with the Model Context Protocol (MCP) Go SDK.

## Overview

The MCP SDK provides a pluggable authentication system that supports multiple authentication methods:
- **Bearer Token Authentication**: For session-based authentication with refresh tokens
- **API Key Authentication**: For long-lived service-to-service authentication
- **Custom Authentication**: Implement your own authentication provider

## Features Demonstrated

### 1. Bearer Token Authentication
- Creating and validating bearer tokens
- Token expiration and refresh
- Session-based authentication flow

### 2. API Key Authentication
- Generating secure API keys
- API key validation
- Rate limiting support (configurable)

### 3. Server with Authentication
- Protecting MCP server endpoints
- Role-based access control (RBAC)
- Permission-based tool filtering

## Running the Example

```bash
go run main.go
```

## Key Concepts

### Authentication Providers

Authentication providers implement the `AuthProvider` interface:

```go
type AuthProvider interface {
    Authenticate(ctx context.Context, req *AuthRequest) (*AuthResult, error)
    Validate(ctx context.Context, token string) (*UserInfo, error)
    Refresh(ctx context.Context, refreshToken string) (*AuthResult, error)
    Revoke(ctx context.Context, token string) error
    Type() string
}
```

### Transport Configuration

Enable authentication in your transport configuration:

```go
config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
config.Features.EnableAuthentication = true
config.Security.Authentication = &transport.AuthenticationConfig{
    Type:         "bearer",
    Required:     true,
    TokenExpiry:  10 * time.Minute,
    EnableCache:  true,
    CacheTTL:     5 * time.Minute,
}
```

### Using Authentication Context

Pass authentication tokens via context:

```go
ctx := auth.ContextWithToken(context.Background(), token)
result, err := client.SendRequest(ctx, "method", params)
```

### Role-Based Access Control (RBAC)

The authentication system supports RBAC with configurable permissions:

```go
config.Security.Authentication.RBAC = &auth.RBACConfig{
    Enabled:     true,
    DefaultRole: "user",
    ResourcePermissions: map[string][]string{
        "admin_tool": {"admin"},
        "write_data": {"write"},
        "read_data":  {"read"},
    },
}
```

## Security Best Practices

1. **Always use HTTPS** in production for HTTP transports
2. **Implement token expiration** and refresh mechanisms
3. **Use secure random generation** for tokens and API keys
4. **Enable token caching** to improve performance
5. **Implement rate limiting** for API endpoints
6. **Log authentication events** for security auditing

## Production Considerations

### Token Storage
In production, replace the in-memory token storage with:
- Redis for distributed systems
- Database for persistent storage
- Secure key management service

### Authentication Providers
Consider using established authentication services:
- OAuth2/OIDC providers (Auth0, Okta, etc.)
- JWT token validation
- LDAP/Active Directory integration

### Security Headers
For HTTP transports, configure security headers:
- CORS policies
- Rate limiting
- Request size limits
- TLS configuration

## Advanced Features

### Custom Authentication
Implement custom authentication by creating your own `AuthProvider`:

```go
type CustomAuthProvider struct {
    // Your implementation
}

func (p *CustomAuthProvider) Authenticate(ctx context.Context, req *AuthRequest) (*AuthResult, error) {
    // Custom authentication logic
}
```

### Token Caching
The authentication middleware includes built-in token caching:
- Reduces validation overhead
- Configurable TTL and size limits
- Automatic cleanup of expired entries

### Middleware Integration
Authentication integrates seamlessly with other middleware:
- Reliability (retries, circuit breakers)
- Observability (metrics, logging)
- Rate limiting

## Troubleshooting

### Common Issues

1. **Import cycle errors**: Make sure to import the auth package to register the middleware factory
2. **Token validation failures**: Check token expiration and format
3. **Permission denied**: Verify user roles and permissions configuration

### Debug Logging

Enable debug logging to troubleshoot authentication issues:

```go
config.Observability.EnableLogging = true
config.Observability.LogLevel = "debug"
```
