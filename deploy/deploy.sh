#!/bin/sh
# Thanks @kyokomi

APP_NAME=intertube
S3_REGION=us-west-2
S3_BUCKET=deploy.inter.tube
CIRCLE_BUILD_NUM=555

TAR_NAME=${APP_NAME}.zip

# s3 upload
aws s3 cp --region $S3_REGION $TAR_NAME s3://$S3_BUCKET/$APP_NAME/release/$CIRCLE_BUILD_NUM/$TAR_NAME

# lambda deploy
aws lambda update-function-code --region $S3_REGION --function-name "tube-web" --s3-bucket $S3_BUCKET --s3-key $APP_NAME/release/$CIRCLE_BUILD_NUM/$TAR_NAME 
aws lambda update-function-code --region $S3_REGION --function-name "tube-trigger" --s3-bucket $S3_BUCKET --s3-key $APP_NAME/release/$CIRCLE_BUILD_NUM/$TAR_NAME 
aws lambda update-function-code --region $S3_REGION --function-name "tube-refresh" --s3-bucket $S3_BUCKET --s3-key $APP_NAME/release/$CIRCLE_BUILD_NUM/$TAR_NAME 
