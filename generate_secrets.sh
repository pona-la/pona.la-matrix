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

/bin/echo -n 'OOYE_HS_TOKEN="'
dd if=/dev/urandom bs=32 count=1 2> /dev/null | basenc --base16 | dd conv=lcase 2> /dev/null
echo '"'
/bin/echo -n 'OOYE_HS_TOKEN="'
dd if=/dev/urandom bs=32 count=1 2> /dev/null | basenc --base16 | dd conv=lcase 2> /dev/null
echo '"'
