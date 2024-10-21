const pipeline = () => {
  try {
    run({
      name: 'simple-task',
      image: 'alpine',
      command: ['echo', 'Hello, World!'],
    })
  } catch(error) {
    console.log("Something unexpected occurred: ", error);
  }
};

export { pipeline };