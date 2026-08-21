param(
    [Parameter(Mandatory = $true)]
    [string]$DatabaseUrl,
    [switch]$Seed
)

$ErrorActionPreference = 'Stop'
psql $DatabaseUrl -v ON_ERROR_STOP=1 -f "$PSScriptRoot\..\migrations\001_initial.sql"
if ($Seed) {
    psql $DatabaseUrl -v ON_ERROR_STOP=1 -f "$PSScriptRoot\..\migrations\002_seed_demo.sql"
}
