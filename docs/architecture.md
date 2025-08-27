# TAMS Oracle Cloud Infrastructure Architecture

## Overview

The TAMS (Television Archive Management System) infrastructure on Oracle Cloud Infrastructure (OCI) is designed to provide a scalable, secure, and highly available platform for managing broadcast media archives.

## Architecture Diagram

```mermaid
graph TB
    subgraph Internet
        Users[Users/Clients]
    end
    
    subgraph OCI[Oracle Cloud Infrastructure]
        subgraph VCN[Virtual Cloud Network - 10.0.0.0/16]
            subgraph PublicSubnet[Public Subnet - 10.0.1.0/24]
                LB[Load Balancer<br/>SSL Termination]
                IGW[Internet Gateway]
            end
            
            subgraph PrivateSubnet[Private Subnet - 10.0.2.0/24]
                API1[TAMS API<br/>Instance 1]
                API2[TAMS API<br/>Instance 2]
                APIN[TAMS API<br/>Instance N<br/>Auto-scaling]
                NAT[NAT Gateway]
            end
            
            subgraph DBSubnet[Database Subnet - 10.0.3.0/24]
                DB[(PostgreSQL<br/>Database)]
                DBVol[DB Volume<br/>500GB]
            end
            
            SGW[Service Gateway]
        end
        
        subgraph Storage[Object Storage]
            MediaBucket[Media Bucket<br/>Standard Tier<br/>7 Year Retention]
            ArchiveBucket[Archive Bucket<br/>Archive Tier<br/>10 Year Retention]
            TempBucket[Temp Bucket<br/>Standard Tier<br/>7 Day Lifecycle]
        end
        
        subgraph Management[Management Services]
            AutoScale[Auto Scaling<br/>Configuration]
            Backup[Volume Backup<br/>Policy]
            IAM[IAM Policies]
        end
    end
    
    Users --> LB
    LB --> API1
    LB --> API2
    LB --> APIN
    
    API1 --> DB
    API2 --> DB
    APIN --> DB
    
    API1 --> MediaBucket
    API2 --> ArchiveBucket
    APIN --> TempBucket
    
    DB -.-> DBVol
    
    PublicSubnet --> IGW
    PrivateSubnet --> NAT
    DBSubnet --> SGW
    
    AutoScale --> APIN
    Backup --> DBVol
    IAM --> Storage
```

## Network Architecture

```mermaid
graph LR
    subgraph Network Topology
        VCN[VCN<br/>10.0.0.0/16]
        
        VCN --> PS[Public Subnet<br/>10.0.1.0/24]
        VCN --> PRS[Private Subnet<br/>10.0.2.0/24]
        VCN --> DBS[DB Subnet<br/>10.0.3.0/24]
        
        PS --> RT1[Public Route Table]
        PRS --> RT2[Private Route Table]
        DBS --> RT2
        
        RT1 --> IGW[Internet Gateway<br/>0.0.0.0/0]
        RT2 --> NAT[NAT Gateway<br/>0.0.0.0/0]
        RT2 --> SGW[Service Gateway<br/>OCI Services]
        
        PS --> SL1[Public Security List<br/>443, 80, 22]
        PRS --> SL2[Private Security List<br/>8080, 22]
        DBS --> SL3[DB Security List<br/>5432]
    end
```

## Data Flow Diagram

```mermaid
sequenceDiagram
    participant Client
    participant LB as Load Balancer
    participant API as TAMS API
    participant DB as PostgreSQL
    participant OS as Object Storage
    
    Client->>LB: HTTPS Request
    LB->>API: Forward Request
    
    alt Upload Media
        API->>OS: Store 5-min chunks
        API->>DB: Update metadata
        API-->>Client: Upload confirmation
    else Query Archive
        API->>DB: Query metadata
        DB-->>API: Return results
        API->>OS: Get object URLs
        API-->>Client: Return media info
    else Process Media
        API->>OS: Read from temp bucket
        API->>API: Process chunks
        API->>OS: Write to media bucket
        API->>DB: Update status
        API-->>Client: Processing complete
    end
```

## Storage Architecture

```mermaid
graph TD
    subgraph Storage Tiers
        Upload[Media Upload] --> Temp[Temp Bucket<br/>Immediate Access]
        Temp --> Process[Processing]
        Process --> Media[Media Bucket<br/>Standard Tier]
        Media --> Archive[Archive Bucket<br/>Archive Tier]
        
        Temp --> LC1[7-day Lifecycle Policy]
        Media --> LC2[7-year Retention]
        Archive --> LC3[10-year Retention]
        Archive --> AT[Auto-tiering<br/>Infrequent Access]
    end
```

## Security Architecture

```mermaid
graph TB
    subgraph Security Layers
        subgraph Network Security
            FW[OCI Security Lists]
            NSG[Network Security Groups]
            SSL[SSL/TLS Termination]
        end
        
        subgraph Identity & Access
            IAM[IAM Policies]
            Groups[User Groups]
            Roles[Instance Principals]
        end
        
        subgraph Data Security
            ENC1[Encryption at Rest]
            ENC2[Encryption in Transit]
            BACKUP[Automated Backups]
        end
        
        subgraph Compliance
            RET[Retention Policies]
            AUDIT[Audit Logging]
            VER[Object Versioning]
        end
    end
```

## Deployment Process

```mermaid
stateDiagram-v2
    [*] --> Initialize: terraform init
    Initialize --> Plan: terraform plan
    Plan --> Validate: Review changes
    Validate --> Apply: terraform apply
    Apply --> Provision: Create resources
    
    state Provision {
        [*] --> Network: Create VCN & Subnets
        Network --> Security: Configure Security Lists
        Security --> Storage: Create Object Buckets
        Storage --> Database: Deploy PostgreSQL
        Database --> Compute: Launch API Instances
        Compute --> LoadBalancer: Configure LB
        LoadBalancer --> AutoScale: Setup Auto-scaling
        AutoScale --> [*]
    }
    
    Provision --> Verify: Health checks
    Verify --> [*]: Deployment complete
```

## Scaling Strategy

```mermaid
graph LR
    subgraph Auto-scaling Configuration
        Metrics[CPU Metrics] --> Threshold{Threshold Check}
        Threshold -->|CPU > 80%| ScaleOut[Scale Out<br/>+2 instances]
        Threshold -->|CPU < 20%| ScaleIn[Scale In<br/>-1 instance]
        
        ScaleOut --> Pool[Instance Pool<br/>Min: 2, Max: 10]
        ScaleIn --> Pool
        
        Pool --> Cooldown[5-min Cooldown]
        Cooldown --> Metrics
    end
```

## Backup and Recovery

```mermaid
graph TD
    subgraph Backup Strategy
        DB[(PostgreSQL)] --> Daily[Daily Incremental<br/>7-day retention]
        DB --> Weekly[Weekly Full<br/>30-day retention]
        
        Daily --> VolumeBackup[Volume Snapshots]
        Weekly --> VolumeBackup
        
        DB --> Export[SQL Export]
        Export --> ObjectStorage[Archive Bucket]
        
        VolumeBackup --> Recovery{Recovery Scenario}
        Recovery -->|Point-in-time| Restore1[Restore from Snapshot]
        Recovery -->|Disaster| Restore2[Restore from Archive]
    end
```

## Monitoring and Observability

```mermaid
graph TB
    subgraph Monitoring Stack
        Components[Infrastructure Components]
        
        Components --> Metrics[OCI Metrics]
        Components --> Logs[OCI Logging]
        Components --> Events[OCI Events]
        
        Metrics --> Alarms[Threshold Alarms]
        Logs --> Analytics[Log Analytics]
        Events --> Notifications[Email/SMS Alerts]
        
        subgraph Health Checks
            LBHealth[Load Balancer Health]
            APIHealth[API /health endpoint]
            DBHealth[Database Health]
        end
        
        LBHealth --> Dashboard[Monitoring Dashboard]
        APIHealth --> Dashboard
        DBHealth --> Dashboard
    end
```

## Cost Optimization

```mermaid
pie title Storage Cost Distribution
    "Media Bucket (Standard)" : 40
    "Archive Bucket (Archive)" : 20
    "Temp Bucket (Standard)" : 5
    "Database Volume" : 15
    "Compute Instances" : 20
```

## Disaster Recovery Plan

```mermaid
graph LR
    subgraph Primary Region
        PrimaryVCN[Production VCN]
        PrimaryDB[(Primary DB)]
        PrimaryStorage[Object Storage]
    end
    
    subgraph DR Region
        DRVCN[DR VCN]
        DRDB[(Standby DB)]
        DRStorage[Replicated Storage]
    end
    
    PrimaryDB -.->|Async Replication| DRDB
    PrimaryStorage -.->|Cross-region Copy| DRStorage
    
    subgraph Failover Process
        Detect[Detect Failure] --> Verify[Verify DR Ready]
        Verify --> DNS[Update DNS]
        DNS --> Activate[Activate DR]
    end
```

## Key Features

### High Availability
- Multi-instance API deployment behind load balancer
- Auto-scaling based on CPU metrics
- Database with automated backups

### Security
- Network isolation with private subnets
- SSL/TLS termination at load balancer
- IAM policies for resource access control
- Encryption at rest and in transit

### Scalability
- Horizontal scaling of API instances
- Object storage for unlimited media storage
- Flexible compute shapes

### Cost Optimization
- Archive tier for long-term storage
- Auto-tiering for infrequent access
- Lifecycle policies for temporary data

## Deployment Requirements

1. OCI Account with appropriate permissions
2. Configured OCI CLI or API keys
3. SSL certificates for HTTPS
4. SSH key pair for instance access
5. Terraform >= 1.0

## Maintenance Windows

- Database backups: Daily at 2 AM UTC
- System updates: Sunday 3 AM UTC
- Health checks: Every 5 minutes