# WinGet submission

The `0.3.0` directory contains the current multi-file manifest for
`MartinTrifunov.ThisYou`. Validate and test it from Windows before submission:

```powershell
winget validate --manifest .\packaging\winget\0.3.0
winget settings --enable LocalManifestFiles
winget install --manifest .\packaging\winget\0.3.0
```

To submit it to the Microsoft community package repository without GitHub CLI:

```powershell
winget install wingetcreate
wingetcreate token -s
wingetcreate submit .\packaging\winget\0.3.0
```

Submission opens a pull request in `microsoft/winget-pkgs`. Future releases need
a new version directory with version-specific release URLs and SHA-256 hashes.
