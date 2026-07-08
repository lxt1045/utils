# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Package Overview

`geohash` is a high-performance Go implementation for encoding geographic coordinates (latitude/longitude) into geohash strings and integers. It includes AMD64-specific optimizations using BMI2 instructions (PDEP).

**Key capabilities:**
- Encode coordinates to geohash string (`wx4g0ec19x3d`) or uint64
- Decode geohash back to coordinates
- Calculate 8 neighboring geohash cells
- Calculate distance between two coordinates
- Two implementation paths: standard bit manipulation and BMI2-optimized (PDEP instruction)

## Testing & Benchmarking

```bash
# Run all tests (compares results against mmcloughlin/geohash for correctness)
go test -v

# Run specific test
go test -run TestNeighborsInt

# Benchmark against other geohash libraries
go test -bench=. -benchmem

# Benchmark specific function
go test -bench=BenchmarkEncode3
```

**Important:** Many tests use `t.Error()` instead of `t.Fatal()` - this is intentional for logging comparison output, not actual failures.

## Architecture

### Coordinate Encoding Flow
```
(lat, lng) float64
    ↓ EncodeCoords()
(x, y) uint32  (32-bit quantized coordinates)
    ↓ interleave64() or interleave64_2()
uint64  (bits interleaved: even=lat, odd=lng)
    ↓ Geo2Str()
"wx4g0ec19x3d"  (base32 string)
```

### Two Interleaving Implementations

1. **Standard (`interleave64`)**: Portable bit manipulation using shifts and masks
2. **BMI2-optimized (`interleave64_2`)**: Uses Intel/AMD BMI2 `PDEP` instruction via assembly
   - Requires AMD64 CPU with BMI2 support
   - Assembly in `pdep_amd64.s`, Go wrapper in `pdep_amd64.go`
   - Check `cpu.X86.HasBMI2` at runtime

The `PDEP` (Parallel Bits Deposit) instruction scatters source bits according to a mask - perfect for interleaving coordinate bits.

### Neighbor Calculation

Two implementations:
- `NeighborsInt()`: Deinterleaves to (lat, lng), adds delta, re-interleaves
- `NeighborsInt2()`: Operates directly on interleaved bits without deinterleaving (faster)

Both return 8 neighbors in order: N, NE, E, SE, S, SW, W, NW

## Key Functions

- `Encode(lat, lng)` → 12-char geohash string
- `EncodeInt(lat, lng)` → uint64 geohash
- `EncodeCoords(lat, lng)` → (x, y) uint32 quantized coordinates
- `Decode(str)` / `DecodeInt(geo)` → (lat, lng)
- `NeighborsInt(geo, bits)` → []uint64 of 8 neighbors
- `Dist(p1, p2)` → distance in km (Euclidean approximation on sphere)

## Constants & Precision

- Coordinates quantized to 32 bits each (total 64 bits when interleaved)
- `deltaAngle = 0.0000000000001` prevents overflow at (±90, ±180)
- 12-character geohash = 60 bits (5 bits per base32 char)
- Base32 alphabet: `0123456789bcdefghjkmnpqrstuvwxyz` (no 'a', 'i', 'l', 'o')

## Performance Notes

- BMI2 path (`EncodeInt2`) is ~2x faster on supported CPUs
- `NeighborsInt2` avoids deinterleave overhead
- Benchmarks compare against 7+ popular geohash libraries
- Zero allocations for core encode/decode operations
