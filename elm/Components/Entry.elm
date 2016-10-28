module Entry exposing (..)

import Json.Decode as Decode
import Json.Encode as Encode
import Json.Decode.Pipeline as Pipeline
import Generic
import Html


type alias Model =
    { fetchdate : String
    , hookdate : String
    , resource : Generic.Value
    , sha1 : String
    }


{-|
   pipeline decoder for our Model
   https://www.brianthicks.com/post/2016/08/22/decoding-large-json-objects-a-summary/
-}
decode : Decode.Decoder Model
decode =
    Pipeline.decode Model
        |> Pipeline.required "fetchdate" Decode.string
        |> Pipeline.required "hookdate" Decode.string
        |> Pipeline.required "resource" Generic.decoder
        |> Pipeline.required "sha1" Decode.string


render : Model -> Html.Html a
render model =
    let
        li =
            Html.li

        text =
            Html.text
    in
        Html.div []
            [ Html.ul []
                [ li [] [ text <| "SHA1: " ++ model.sha1 ]
                , li [] [ text <| "Fetch Date: " ++ model.fetchdate ]
                , li [] [ text <| "Hook Date: " ++ model.hookdate ]
                ]
            , Html.pre []
                [ model.resource
                    |> Generic.toJson
                    |> Encode.encode 4
                    |> text
                ]
            ]
