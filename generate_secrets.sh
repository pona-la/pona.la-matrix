#!/bin/sh

print_random () {
  LC_ALL=C tr -dc 'A-Za-z0-9!#%&()*+,-./:;<=>?@[\]^_{|}~' </dev/urandom | head -c 32
}

/bin/echo -n 'LDAP_JWT_SECRET="'
print_random
echo '"'
/bin/echo -n 'LDAP_KEY_SEED="'
print_random
echo '"'