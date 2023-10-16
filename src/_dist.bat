pyinstaller "gui.py" --onefile --clean --distpath "" --icon="app/icon.ico" --add-data="resources;resources" --add-data="icons;icons" --add-data="app;app"
@RD /S /Q "build"
rename gui.exe pscoverdl.exe