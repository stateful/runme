# AI Services

This directory contains the protos to define the services used to interact with
the AI modules. This service is used to call [Foyle AI](https://foyle.io) to
generate cells for the notebook.

By design the generated go code is in its own module. This makes it easier for
Foyle and other projects to reuse the service definition without pulling in
the entire codebase.
