package main

const portal = `
<!DOCTYPE html>
<html lang='en'>
    <style>
        h1 {
            text-align:center;
        }
    </style>
    <body>
        <h1>
        <form action="/dict?format=html" method="get">
        <label for="word">Query:</label>
        <input type="text" id="name" name="query" placeholder="doctor" required/><br>
        <label for="word">Engine:</label>
        <input type="text" id="engine" name="engine" value="mdx" placeholder="mdx"/><br>
        <!-- <label for="word">Format:</label> -->
        <input type="hidden" name="format" value="html" />

        <!-- <label for="message">Message:</label> -->
        <!-- <textarea id="message" name="message" rows="4" cols="30"></textarea><br> -->
        <input type="submit" value="Submit"/>
        </form>
        </h1>
    </body>
</html>
`
