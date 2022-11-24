import * as React from 'react';
import * as ReactDOM from 'react-dom';

import CssBaseline from '@mui/material/CssBaseline';
import Grid from '@mui/material/Unstable_Grid2';
import Paper from '@mui/material/Paper';

import Stepper from './stepper';

import '@fontsource/roboto/300.css';
import '@fontsource/roboto/400.css';
import '@fontsource/roboto/500.css';
import '@fontsource/roboto/700.css';
import { client } from './client';

function App() {

  React.useEffect(() => {
    async function fetchData() {
       const response = await client.cheeer({});
    }

    fetchData();
  }, []);

  return (
    <React.Fragment>
        <CssBaseline enableColorScheme />
        <Grid container spacing={2}>
          <Grid xs={12} sm={6} smOffset={3}>
            <Paper>
              Hello world, my darling
              <Stepper />
            </Paper>
          </Grid>
        </Grid>
    </React.Fragment>
  );
}

ReactDOM.render(<App />, document.getElementById("root"));
