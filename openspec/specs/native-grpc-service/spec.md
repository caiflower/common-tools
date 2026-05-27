## Requirements

### Requirement: Register native gRPC service on cluster server
The cluster SHALL provide a `RegisterGRPCService(sd *grpc.ServiceDesc, ss interface{})` method that registers a user-defined gRPC service on the cluster's gRPC server. The service SHALL be accessible on the same port as the cluster's internal `ClusterService`.

#### Scenario: Register service before cluster starts
- **WHEN** user calls `RegisterGRPCService(sd, impl)` before calling `Start()`
- **THEN** the service SHALL be registered on the gRPC server when it starts, and clients SHALL be able to call the service methods

#### Scenario: Register service after cluster starts
- **WHEN** user calls `RegisterGRPCService(sd, impl)` after calling `Start()`
- **THEN** the service SHALL be dynamically registered on the running gRPC server via `grpc.Server.RegisterService()`, and clients SHALL be able to call the service methods immediately

#### Scenario: Register duplicate service
- **WHEN** user calls `RegisterGRPCService` with a service name that is already registered
- **THEN** the cluster SHALL return an error indicating the service is already registered

### Requirement: Get gRPC client connection for a node
The cluster SHALL provide a `GetGRPCClient(nodeName string) (grpc.ClientConnInterface, error)` method that returns the gRPC client connection for the specified node. The returned connection SHALL be reusable for creating any gRPC client stub.

#### Scenario: Get connection for alive node
- **WHEN** user calls `GetGRPCClient(nodeName)` with a node that is in `aliveNodes` and has an active gRPC connection
- **THEN** the method SHALL return the node's `grpc.ClientConnInterface` and no error

#### Scenario: Get connection for unknown node
- **WHEN** user calls `GetGRPCClient(nodeName)` with a node name that does not exist in the cluster
- **THEN** the method SHALL return `nil` and an error indicating the node does not exist

#### Scenario: Get connection for unreachable node
- **WHEN** user calls `GetGRPCClient(nodeName)` with a node that exists but has no active gRPC connection
- **THEN** the method SHALL return `nil` and an error indicating the node is unreachable

#### Scenario: Use connection to create native gRPC client stub
- **WHEN** user obtains a connection via `GetGRPCClient` and creates a gRPC client stub (e.g., `proto.NewMyServiceClient(conn)`)
- **THEN** the stub SHALL be able to call RPC methods on the target node's registered gRPC service

### Requirement: Connection lifecycle management
The gRPC connections returned by `GetGRPCClient` SHALL be managed by the cluster. The connections SHALL be closed when the cluster is closed.

#### Scenario: Cluster close invalidates connections
- **WHEN** the cluster is closed via `Close()`
- **THEN** all gRPC connections returned by `GetGRPCClient` SHALL be closed, and any pending RPC calls on those connections SHALL fail with a context canceled error

#### Scenario: Connection follows node availability
- **WHEN** a node becomes unavailable (marked as lost) and then reconnects
- **THEN** the connection returned by `GetGRPCClient` SHALL reflect the current state of the node's gRPC connection
