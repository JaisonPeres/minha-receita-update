# Limited Disk Space Configuration

If you have limited disk space and want to run Minha Receita with a smaller dataset, this guide shows how to configure it.

## Overview

The full CNPJ database requires approximately:
- **~140GB** for database storage
- **~15GB** for temporary processing
- **Total: ~170GB** available disk space

For systems with limited disk space, you can use the `--max-records` flag to limit the number of company records saved to the database.

## Quick Start

### Option 1: Using Sample Data (Recommended for Testing)

The fastest way to test with limited data:

```bash
# 1. Create sample files (10,000 lines per file)
docker-compose run --rm minha-receita sample -d /mnt/data/

# 2. Transform only the sample data
docker-compose run --rm minha-receita transform -d data/sample/

# Result: ~50MB database, processes in ~1 minute
```

### Option 2: Using Max Records Limit

Process the full download but limit the database size:

```bash
# 1. Download all data (~10GB)
docker-compose run --rm minha-receita download -d /mnt/data/

# 2. Transform with record limit (e.g., 100,000 companies)
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 100000

# Result: ~1GB database instead of 140GB
```

## Disk Space by Record Count

Approximate database sizes for different record limits:

| Max Records | Database Size | Use Case |
|-------------|---------------|----------|
| 10,000 | ~50 MB | Testing/Development |
| 50,000 | ~250 MB | Small demos |
| 100,000 | ~500 MB | Medium demos |
| 500,000 | ~2.5 GB | Regional subset |
| 1,000,000 | ~5 GB | Large subset |
| Unlimited (0) | ~140 GB | Full production |

## Usage Examples

### Basic Usage

```bash
# Limit to 10,000 records
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 10000

# Limit to 100,000 records
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 100000

# No limit (process all records)
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 0
# or simply omit the flag:
docker-compose run --rm minha-receita transform -d /mnt/data/
```

### Combined with Other Options

```bash
# With custom batch size (reduces memory usage)
docker-compose run --rm minha-receita transform \
  -d /mnt/data/ \
  --max-records 50000 \
  --batch-size 4096

# With cleanup (drop & recreate tables first)
docker-compose run --rm minha-receita transform \
  -d /mnt/data/ \
  --max-records 100000 \
  --clean-up

# With custom parallel queries (adjust for your hardware)
docker-compose run --rm minha-receita transform \
  -d /mnt/data/ \
  --max-records 50000 \
  --max-parallel-db-queries 4
```

## How It Works

The `--max-records` flag:
1. **Processes lookup tables completely** (Cnaes, Municipios, Qualificações, etc.)
2. **Stops saving company records** after reaching the limit
3. **Still creates indexes** on the saved data
4. **Saves the updated_at metadata** normally

### What Gets Limited

- ✅ Company records (Estabelecimentos) - **LIMITED**
- ❌ Lookup tables (CNAEs, cities, qualifications, etc.) - **ALL processed** (needed for references)
- ❌ Indexes - **ALL created** (for performance)

### Important Notes

- Records are processed in the order they appear in the source files
- There's no filtering by state, city, or other criteria (it's a simple count limit)
- The temporary badger storage still processes all files but only saves the limited records to the database
- **Progress bar shows your limit** - not the total source records (when limit is set)
- **Processing stops immediately** when limit is reached - saves processing time!

## Disk Space Savings

### Before (Full Database)
```
Source files (download):     ~10 GB
Temporary storage (badger):  ~15 GB
Database (PostgreSQL):       ~140 GB
Indexes:                     ~10 GB
-----------------------------------
TOTAL:                       ~175 GB
```

### After (With 100k Records Limit)
```
Source files (download):     ~10 GB
Temporary storage (badger):  ~15 GB (same)
Database (PostgreSQL):       ~500 MB ⬅️ REDUCED!
Indexes:                     ~50 MB ⬅️ REDUCED!
-----------------------------------
TOTAL:                       ~26 GB
```

**Savings: ~149 GB (85% reduction)**

## Recommendations

### For Development/Testing
```bash
# Use sample command (fastest)
docker-compose run --rm minha-receita sample -d /mnt/data/
docker-compose run --rm minha-receita transform -d data/sample/
```

### For Limited Disk Space (<50GB available)
```bash
# Use max-records with 50k-100k limit
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 50000
```

### For Production
```bash
# No limit (process all records)
docker-compose run --rm minha-receita transform -d /mnt/data/
```

## Monitoring Progress

The transform command shows progress with adjusted totals:

**Without limit:**
```
Creating the JSON data for each CNPJ   5% |████                | (3500000/69200268, 10372 it/s)
```

**With limit (e.g., --max-records 100000):**
```
Creating the JSON data for each CNPJ (limited to 100000 records)   45% |████████    | (45000/100000, 8500 it/s)
[INFO] Reached maximum records limit max=100000 processed=100000
```

When the limit is reached:
1. Progress bar shows your limit (not the total in source files)
2. Description indicates it's limited
3. Processing stops immediately
4. Log message confirms the limit was reached

## Clearing Previous Data

If you want to try different limits:

```bash
# Drop existing database
docker-compose run --rm minha-receita drop

# Recreate tables
docker-compose run --rm minha-receita create

# Transform with new limit
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 50000
```

Or use `--clean-up` to do it in one command:
```bash
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 50000 --clean-up
```

## Combining with Sample Command

For the smallest footprint:

```bash
# 1. Create sample with custom line limit
docker-compose run --rm minha-receita sample -d /mnt/data/ --max-lines 5000

# 2. Transform sample data with record limit
docker-compose run --rm minha-receita transform -d data/sample/ --max-records 5000

# Result: ~25MB database, processes in ~30 seconds
```

## Troubleshooting

### Still Running Out of Space?

1. **Reduce batch size** (uses less memory):
   ```bash
   --batch-size 2048
   ```

2. **Clean up temporary files** after transform:
   ```bash
   docker-compose run --rm minha-receita cleanup
   ```

3. **Mount data directory to external storage**:
   ```yaml
   # docker-compose.yml
   volumes:
     - /external/drive/data:/mnt/data
   ```

### Want to Process Specific States/Cities?

Currently, `--max-records` is a simple count limit. For filtering by state or city, you would need to:
1. Process with a higher limit
2. Filter records in your application layer
3. Or wait for a future feature update

## Performance Impact

Using `--max-records`:
- ✅ **Significantly reduces database size**
- ✅ **Stops processing early** when limit is reached
- ✅ **Reduces overall time** compared to full processing
- ⚠️ **Still reads all source files** (until limit is reached)
- ⚠️ **Still processes lookup tables** completely

For the absolute fastest processing, use the `sample` command instead.
