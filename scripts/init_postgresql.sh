#!/bin/bash
set -e

DB_PASSWORD="${db_password}"
DB_VERSION="${db_version}"
BACKUP_BUCKET="${backup_bucket_name}"
NAMESPACE="${namespace}"

sudo apt-get update
sudo apt-get install -y postgresql-${DB_VERSION} postgresql-client-${DB_VERSION} postgresql-contrib-${DB_VERSION}

sudo systemctl stop postgresql

sudo mkdir -p /data/postgresql
sudo chown postgres:postgres /data/postgresql

echo "data_directory = '/data/postgresql/${DB_VERSION}/main'" | sudo tee -a /etc/postgresql/${DB_VERSION}/main/postgresql.conf

sudo -u postgres /usr/lib/postgresql/${DB_VERSION}/bin/initdb -D /data/postgresql/${DB_VERSION}/main

cat <<EOF | sudo tee -a /etc/postgresql/${DB_VERSION}/main/postgresql.conf
listen_addresses = '*'
max_connections = 200
shared_buffers = 8GB
effective_cache_size = 24GB
maintenance_work_mem = 2GB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1
work_mem = 20971kB
min_wal_size = 2GB
max_wal_size = 8GB
max_worker_processes = 4
max_parallel_workers_per_gather = 2
max_parallel_workers = 4
max_parallel_maintenance_workers = 2

archive_mode = on
archive_command = 'test ! -f /backup/%f && cp %p /backup/%f'
wal_level = replica
max_wal_senders = 3
wal_keep_size = 1GB
EOF

echo "host    all             all             10.0.0.0/16            md5" | sudo tee -a /etc/postgresql/${DB_VERSION}/main/pg_hba.conf

sudo systemctl start postgresql
sudo systemctl enable postgresql

sudo -u postgres psql <<EOF
ALTER USER postgres PASSWORD '${DB_PASSWORD}';
CREATE DATABASE tamsdb;
CREATE USER tamsapi WITH ENCRYPTED PASSWORD '${DB_PASSWORD}';
GRANT ALL PRIVILEGES ON DATABASE tamsdb TO tamsapi;
EOF

sudo mkdir -p /backup
sudo chown postgres:postgres /backup

cat <<'BACKUP_SCRIPT' | sudo tee /usr/local/bin/backup_postgres.sh
#!/bin/bash
BACKUP_DIR="/backup"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/tamsdb_backup_$TIMESTAMP.sql"

sudo -u postgres pg_dump tamsdb > $BACKUP_FILE
gzip $BACKUP_FILE

oci os object put --bucket-name ${BACKUP_BUCKET} --file $BACKUP_FILE.gz --namespace ${NAMESPACE}

find $BACKUP_DIR -name "*.gz" -mtime +7 -delete
BACKUP_SCRIPT

sudo chmod +x /usr/local/bin/backup_postgres.sh

echo "0 2 * * * /usr/local/bin/backup_postgres.sh" | sudo crontab -

sudo apt-get install -y python3-pip
pip3 install oci-cli

echo "PostgreSQL initialization complete"