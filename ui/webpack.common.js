const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
    entry: {
        app: path.resolve(__dirname, 'src/index.tsx'),
    },
    target: 'web',
    devServer: {
        hot: true,
        compress: true,
        watchFiles: ['src/**/*.ts', 'public/**/*', 'src/**/*.tsx'],
        historyApiFallback: true,
    },
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                use: 'ts-loader',
                exclude: /node_modules/,
            },
            {
                test: /\.m?js$/,
                exclude: /node_modules/,
                use: {
                    loader: 'babel-loader',
                    options: {
                        presets: [
                            "@babel/preset-react",
                            '@babel/preset-env',
                        ]
                    }
                }
            },
            {
                test: /\.svg/,
                use: {
                    loader: "svg-url-loader",
                    options: {
                        // make all svg images to work in IE
                        iesafe: true,
                    },
                },
            },
            {
                test: /\.css$/i,
                use: ["style-loader", "css-loader"],
            },
            {
                test: /\.(png|jpe?g|gif)$/i,
                use: [
                    {
                        loader: 'file-loader',
                    },
                ],
            },
        ],
    },
    plugins: [
        new HtmlWebpackPlugin({
            title: 'Production',
            template: 'public/index.html',
            inject: true
        }),
    ],
    resolve: {
        extensions: ['.js', '.ts', '.tsx', '.css', '.svg', '.png', '.jpeg', '.jpg', '.gif'],
        symlinks: true,
        alias: {
            rpc: path.resolve(__dirname, 'rpc'),
            src: path.resolve(__dirname, 'src')
        }
    },
    output: {
        filename: '[name].[contenthash:8].js',
        sourceMapFilename: '[name].[contenthash:8].map',
        chunkFilename: '[id].[contenthash:8].js',
        path: path.resolve(__dirname, process.env.BUILD_DIR || 'build'),
        publicPath: '/',
        clean: true
    },
    watchOptions: {
        followSymlinks: true,
        poll: true,
        ignored: /node_modules/
    },
};