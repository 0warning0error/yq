
## Decode string
Given a sample.toon file of:
```toon
name: hello
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
name: hello
```

## Decode quoted string
Given a sample.toon file of:
```toon
message: "hello world"
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
message: hello world
```

## Decode number
Given a sample.toon file of:
```toon
count: 42
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
count: 42
```

## Decode float
Given a sample.toon file of:
```toon
pi: 3.14
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
pi: 3.14
```

## Decode boolean true
Given a sample.toon file of:
```toon
enabled: true
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
enabled: true
```

## Decode boolean false
Given a sample.toon file of:
```toon
enabled: false
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
enabled: false
```

## Decode null
Given a sample.toon file of:
```toon
value: null
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
value: null
```

## Decode nested object
Given a sample.toon file of:
```toon
user:
  name: Ada
  id: 123
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
user:
  name: Ada
  id: 123
```

## Decode empty object
Given a sample.toon file of:
```toon
config:
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
config: {}
```

## Decode multiple fields
Given a sample.toon file of:
```toon
name: app
version: 1
enabled: true
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
name: app
version: 1
enabled: true
```

## Decode inline primitive array
Given a sample.toon file of:
```toon
tags[3]: admin,ops,dev
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
tags:
  - admin
  - ops
  - dev
```

## Decode empty array
Given a sample.toon file of:
```toon
items[0]:
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
items: []
```

## Decode number array
Given a sample.toon file of:
```toon
numbers[4]: 1,2,3,4
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
numbers:
  - 1
  - 2
  - 3
  - 4
```

## Decode tabular array
Given a sample.toon file of:
```toon
items[2]{sku,qty,price}:
  A1,2,9.99
  B2,1,14.5
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
items:
  - sku: A1
    qty: 2
    price: 9.99
  - sku: B2
    qty: 1
    price: 14.5
```

## Decode list array with primitives
Given a sample.toon file of:
```toon
items[3]:
  - one
  - two
  - three
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
items:
  - one
  - two
  - three
```

## Decode list array with objects
Given a sample.toon file of:
```toon
users[2]:
  - name: Ada
    id: 1
  - name: Bob
    id: 2
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
users:
  - name: Ada
    id: 1
  - name: Bob
    id: 2
```

## Decode mixed array
Given a sample.toon file of:
```toon
mixed[3]:
  - 1
  - text
  - true
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
mixed:
  - 1
  - text
  - true
```

## Roundtrip simple object
Given a sample.toon file of:
```toon
name: hello
```
then
```bash
yq -otoon sample.toon
```
will output
```toon
name: hello
```

## Roundtrip nested object
Given a sample.toon file of:
```toon
user:
  name: Ada
  id: 123
```
then
```bash
yq -otoon sample.toon
```
will output
```toon
user:
  name: Ada
  id: 123
```

## Roundtrip inline array
Given a sample.toon file of:
```toon
tags[3]: admin,ops,dev
```
then
```bash
yq -otoon sample.toon
```
will output
```toon
tags[3]: admin,ops,dev
```

## Roundtrip tabular array
Given a sample.toon file of:
```toon
items[2]{sku,qty,price}:
  A1,2,9.99
  B2,1,14.5
```
then
```bash
yq -otoon sample.toon
```
will output
```toon
items[2]{sku,qty,price}:
  A1,2,9.99
  B2,1,14.5
```

## Decode complex nested structure
Given a sample.toon file of:
```toon
config:
  database:
    host: localhost
    port: 5432
  cache:
    enabled: true
    ttl: 3600
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
config:
  database:
    host: localhost
    port: 5432
  cache:
    enabled: true
    ttl: 3600
```

## Decode object with nested array
Given a sample.toon file of:
```toon
user:
  name: Ada
  roles[2]: admin,user
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
user:
  name: Ada
  roles:
    - admin
    - user
```

## Decode quoted key
Given a sample.toon file of:
```toon
"key-with-hyphen": value
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
key-with-hyphen: value
```

## Decode string with spaces
Given a sample.toon file of:
```toon
message: "hello world"
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
message: hello world
```

## Decode negative number
Given a sample.toon file of:
```toon
count: -42
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
count: -42
```

## Decode scientific notation
Given a sample.toon file of:
```toon
value: 1e-3
```
then
```bash
yq -oy sample.toon
```
will output
```yaml
value: 1e-3
```

