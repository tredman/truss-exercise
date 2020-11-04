# normalizer

## Requirements

- go 1.13 or higher

## Running

```bash
$ go build
$ ./normalizer < ../sample.csv > sample_normalized.csv
$ ./normalizer < ../sample-with-broken-utf8.csv > sample-with-broken-utf8_normalized.csv
```