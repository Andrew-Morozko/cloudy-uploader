# 1.1.1

- Added darwin/arm64 binary for new M1 computers.
- Nicer progress bar alignment, once again.

# 1.1.0

- Fixed bug with --login and --password not being used
- New command line option to ignore saved credentials
- More documentation
- Configurable ordered submission of uploads to Overcast server:

Overcast used to order the `Uploads` feed by date of submission of the file to the Overcast server.
Because of this `cloudyuploader` submitted the uploaded files in strict order, otherwise they showed up in a mixed up.

While I wasn't looking Marco changed the ordering of `Uploads` feed: now it's ordered by the file name.
However, the upload date still is used in some places. It definitely controls the order of
[Recent Episodes](https://overcast.fm/podcasts) on the web, and may be used in playlist ordering (didn't check that).

Because of that there's a new option `--unordered-submit`. It speeds up uploads somewhat, but file upload dates
become disordered.

- Massive refactoring, now `cloudyuploader` resembles maintainable code.
- Probably some new and exciting bugs. This is a homebrew automation project, not a properly structured application. Sorry 😔

# 1.0.2

- Nicer progress bar alignment

# 1.0.1

- Nicer help message

# 1.0.0

- Now storing passwords in secure platform-dependant storage (macOS Keychain, GNOME Keyring or Windows Credential Manager API)
- Because of that `cloudyuploader` now saves your credentials by default
- Actually silence progress bars if -s is supplied
- Various bugfixes

# 1.0.0-alpha

- Original release
