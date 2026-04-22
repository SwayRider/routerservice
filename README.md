# routerservice

Multi-region routing service for the SwayRider platform. Calculates routes across European regions using Valhalla routing engines, with automatic region assignment and seamless border crossing handling.

## Architecture

The routerservice exposes two server interfaces:

| Interface | Port | Purpose |
| --------- | ---- | ------- |
| REST/HTTP | 8080 | HTTP API via gRPC-gateway |
| gRPC | 8081 | Internal service-to-service communication |

### Dependencies

- **regionservice**: Region lookup and border crossing information
- **Valhalla**: Open-source routing engine (one instance per region)
- **Pelias** (optional): Geocoding service for address resolution

### Multi-Region Routing

The routerservice handles routes that span multiple geographic regions:

1. **Region Assignment**: Determines which regions contain each waypoint using regionservice
2. **Path Planning**: For cross-region routes, finds the optimal sequence of regions
3. **Border Crossings**: Identifies suitable border crossing points based on road type preferences
4. **Route Combination**: Calculates sub-routes within each region and combines them into a seamless response

## Configuration

Configuration is provided via environment variables or CLI flags.

### Server Configuration

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `HTTP_PORT` | `-http-port` | 8080 | REST API port |
| `GRPC_PORT` | `-grpc-port` | 8081 | gRPC port |

### Valhalla Configuration

Valhalla instances can be configured with default naming conventions or explicit per-region hosts/ports.

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `VALHALLA_PREFIX` | `-valhalla-prefix` | valhalla- | Hostname prefix for Valhalla instances |
| `VALHALLA_POSTFIX` | `-valhalla-postfix` | | Hostname postfix for Valhalla instances |
| `VALHALLA_PORT` | `-valhalla-port` | 8002 | Default Valhalla port |
| `VALHALLA_REGION_HOSTS` | `-valhalla-region-hosts` | | Per-region hosts (e.g., "iberian-peninsula:192.168.1.10,west-europe:192.168.1.11") |
| `VALHALLA_REGION_PORTS` | `-valhalla-region-ports` | | Per-region ports (e.g., "iberian-peninsula:33001,west-europe:33002") |

Default hostname pattern: `{prefix}{region-name}{postfix}:{port}`

### Pelias Configuration

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `PELIAS_PREFIX` | `-pelias-prefix` | pelias- | Hostname prefix for Pelias instances |
| `PELIAS_API_POSTFIX` | `-pelias-api-postfix` | -api | Hostname postfix for Pelias API |
| `PELIAS_API_PORT` | `-pelias-api-port` | 3100 | Default Pelias API port |
| `PELIAS_API_REGION_HOSTS` | `-pelias-api-region-hosts` | | Per-region hosts |
| `PELIAS_API_REGION_PORTS` | `-pelias-api-region-ports` | | Per-region ports |

### Service Dependencies

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `REGIONSERVICE_HOST` | `-regionservice-host` | | Region service host |
| `REGIONSERVICE_PORT` | `-regionservice-port` | | Region service port |

## API Reference

The API is defined in the Protocol Buffer files at `backend/protos/router/v1/` and `backend/protos/health/v1/`.

All endpoints are public and require no authentication.

---

### Health Endpoints

#### Ping

Simple health check that returns HTTP 200.

- **Endpoint:** `GET /api/v1/health/ping`
- **Access:** Public

---

### Routing Endpoints

#### Route

Calculates a route between two or more locations.

- **Endpoint:** `POST /api/v1/router/route`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/router/route \
  --header 'content-type: application/json' \
  --data '{
    "id": "route-123",
    "mode": "RM_CAR",
    "resultMode": "RRM_NAVIGATION",
    "locations": [
      {
        "location": {
          "lat": 40.4168,
          "lon": -3.7038
        }
      },
      {
        "location": {
          "lat": 48.8566,
          "lon": 2.3522
        }
      }
    ],
    "routeOptions": {
      "highwayPreference": 0.8,
      "tollwayPreference": 0.5
    }
  }'
```

Response:
```json
{
  "id": "route-123",
  "trip": {
    "status": 0,
    "statusMessage": "Found route between points",
    "unit": "U_METRIC",
    "language": "en-US",
    "locations": [...],
    "legs": [...],
    "summary": {
      "time": 36000,
      "length": 1200.5,
      "hasToll": true,
      "hasHighway": true,
      "hasFerry": false,
      "boundingBox": {...}
    }
  }
}
```

### Request Parameters

#### Routing Modes (`mode`)

| Mode | Description |
| ---- | ----------- |
| `RM_CAR` | Standard car routing |
| `RM_TRUCK` | Truck routing with height/weight restrictions |
| `RM_MOTORCYCLE` | Motorcycle routing |
| `RM_MOTORSCOOTER` | Motor scooter routing (avoids highways) |
| `RM_BICYCLE` | Bicycle routing |
| `RM_PEDESTRIAN` | Walking directions |
| `RM_TRANSIT` | Public transit routing |

#### Result Modes (`resultMode`)

| Mode | Description |
| ---- | ----------- |
| `RRM_NAVIGATION` | Full turn-by-turn navigation with verbal instructions |
| `RRM_DISPLAY_WITH_DETAILS` | Route display with maneuver details |
| `RRM_DISPLAY` | Basic route display |
| `RRM_MINIMAL` | Minimal response (summary only) |

#### Route Options (`routeOptions`)

| Option | Type | Range | Description |
| ------ | ---- | ----- | ----------- |
| `highwayPreference` | float | 0.0-1.0 | Preference for highways (1.0 = prefer) |
| `tollwayPreference` | float | 0.0-1.0 | Preference for toll roads (1.0 = prefer) |
| `primaryPreference` | float | 0.0-1.0 | Preference for primary roads |
| `livingStreetPreference` | float | 0.0-1.0 | Preference for living streets |
| `trackPreference` | float | 0.0-1.0 | Preference for tracks |
| `trailPreference` | float | 0.0-1.0 | Preference for trails |
| `ferryPreference` | float | 0.0-1.0 | Preference for ferries |
| `excludeUnpaved` | bool | | Exclude unpaved roads |
| `shortestPath` | bool | | Optimize for shortest distance |
| `distancePreference` | float | 0.0-1.0 | Balance between time and distance |

### Response Structure

#### Trip

| Field | Description |
| ----- | ----------- |
| `status` | Status code (0 = success) |
| `statusMessage` | Human-readable status |
| `unit` | Distance unit (U_METRIC or U_IMPERIAL) |
| `language` | Response language |
| `locations` | Resolved location details |
| `legs` | Route legs between consecutive waypoints |
| `summary` | Overall trip summary |

#### Leg

| Field | Description |
| ----- | ----------- |
| `shape` | Encoded polyline geometry |
| `elevation` | Elevation data points |
| `elevationInterval` | Distance between elevation samples |
| `maneuvers` | Turn-by-turn instructions |
| `summary` | Leg summary (time, distance, etc.) |

#### Maneuver

| Field | Description |
| ----- | ----------- |
| `type` | Maneuver type (turn, merge, exit, etc.) |
| `instruction` | Written instruction |
| `verbalPreTransitionInstruction` | Verbal instruction before turn |
| `verbalPostTransitionInstruction` | Verbal instruction after turn |
| `streetNames` | Street names for this segment |
| `time` | Time for this maneuver (seconds) |
| `length` | Distance for this maneuver |
| `beginShapeIndex` | Start index in shape polyline |
| `endShapeIndex` | End index in shape polyline |
| `toll` | Is this a toll road |
| `highway` | Is this a highway |
| `lanes` | Lane guidance information |

---

### Ping

Simple endpoint that returns HTTP 200.

- **Endpoint:** `GET /api/v1/router/ping`
- **Access:** Public

## Cross-Region Routing

When a route crosses region boundaries, the service:

1. Identifies the sequence of regions the route passes through
2. Finds optimal border crossing points based on:
   - Road type preferences (motorways preferred for long distances)
   - Distance from the ideal straight-line path
   - Configured drop distance to avoid clustering
3. Calculates sub-routes within each region
4. Merges the sub-routes into a seamless response

### Border Crossing Selection

For cross-region routes, border crossings are selected based on:

- **Road Type**: Motorways and trunk roads preferred for highway-based routing
- **Proximity**: Crossings near the ideal path are preferred
- **Routing Mode**: Motor scooters prefer primary roads over highways

## Error Handling

| Error | Description |
| ----- | ----------- |
| `InvalidArgument` | Missing locations or invalid parameters |
| `InvalidArgument` | Location outside known regions |
| `InvalidArgument` | No route found between points |
| `Internal` | Valhalla or regionservice communication failure |

## Building

```bash
# Generate protobuf code (run from repo root)
make proto

# Build the service
cd backend
go build ./services/routerservice/cmd/routerservice

# Run the service
go run ./services/routerservice/cmd/routerservice
```

## Docker

```bash
# Build container (from repo root)
make services-routerservice-container
```

## Development

For local development with Docker Compose infrastructure:

1. Start base infrastructure: `cd infra/dev/layer-00 && docker-compose up -d`
2. Start geospatial services: `cd infra/dev/layer-10 && docker-compose up -d`
3. Start SwayRider services: `cd infra/dev/layer-20 && docker-compose up -d`

Development ports:
- REST API: 34004
- gRPC: 34104
