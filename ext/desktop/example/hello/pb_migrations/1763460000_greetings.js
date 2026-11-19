/// <reference path="../pb_data/types.d.ts" />
// The greetings collection, open to everyone (a demo), with one greeting to start with.
migrate(
  (app) => {
    const greetings = new Collection({
      type: "base",
      name: "greetings",
      listRule: "",
      viewRule: "",
      createRule: "",
      fields: [
        { name: "text", type: "text", required: true, max: 200 },
        { name: "created", type: "autodate", onCreate: true },
      ],
    })
    app.save(greetings)

    const first = new Record(greetings)
    first.set("text", "Hello, world!")
    app.save(first)
  },
  (app) => {
    app.delete(app.findCollectionByNameOrId("greetings"))
  },
)
