Copy 'n' pasting: ✅🟡❌🤷

# Very basic Init
1. Should check if there already a server in its current folder. If yes, skip a bunch of next stuff, if not continue. ✅
2. Ask if this folder is the folder the server will be in (?) ❌
3. Ask which type of server.jar they want "vanilla", or modded like "spigot" and stuff. ✅
4. Pull the appropiate file from the server of these jars. ✅🟡
5. Create a "run_server.sh" file that will do what it says on the tin. ✅🟡🟡
6. Then the server has to be ran once to intilize the files. ✅
    1. The user must agree to Mojang's EULA. We cannot automate this part. This has to be done via reading the eula.txt file and outputting it to the user. ✅
    2. Server should be ran again after agreeing to the EULA. ✅🤷
    3. The "world" probably has to be removed? We will probably ask the user about world creation since thats one of the main stuff you do with a especially vanilla server.
7. Done! For the init part at least



# Adding new stuff
Quick guides on how to add new elements to the base.

## Adding a new page
1. Add a `stateNewPage` to hold the state of the new page ig.
2. Inside `rootModel`, add `newpage: pages.NewPageModel` to add the new page's model to the model list
3. To intitialize the new page's model when root's model is init,  inside of cobra's `rootCmd` add: `newpage: pages.InitializedNewPageModel()`
4. Inside `Update()`, have a method to switch the new page by setting `m.state = stateNewPage`. The var is indeed from step 1
